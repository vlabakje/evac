package server

import (
	"github.com/miekg/dns"
	"evac/processing"
	"evac/filterlist"
	"time"
)

type Request struct {
	Response dns.ResponseWriter
	Message  *dns.Msg
}

type DnsServer struct {
	IncomingRequests  chan Request
	cache             *processing.Cache
	filter            filterlist.Filter
	recursion_address *string
	worker_amount	  uint16
}

func NewServer(cache *processing.Cache, filter filterlist.Filter, recursion_address *string, worker_amount uint16) (*DnsServer) {
	return &DnsServer{make(chan Request, worker_amount * 10), cache, filter, recursion_address, worker_amount}
}

func (server DnsServer) ServeDNS(writer dns.ResponseWriter, request *dns.Msg) {
	/* Request is handled by a worker with processRequest() */
	server.IncomingRequests <- Request{writer, request}
}

func (server DnsServer) Start(address string) error {
	/* Start configured amount of workers that accept requests from the IncomingRequests channel */
	for i := uint16(0); i < server.worker_amount; i++ {
		go server.acceptRequests()
	}

	/* Listen for DNS requests */
	return dns.ListenAndServe(address, "udp", server)
}

/* Forwards a DNS request to an external DNS server, and returns its result. */
func (server DnsServer) recurse(question dns.Question) (*dns.Msg, time.Duration, error) {
	c := new(dns.Client)
	m := new(dns.Msg)
	m.Id = dns.Id()
	m.RecursionDesired = true
	m.Question = append(m.Question, question)
	return c.Exchange(m, *server.recursion_address)
}

func (server DnsServer) acceptRequests() {
	for true {
		request := <- server.IncomingRequests
		server.processRequest(request.Response, request.Message)
	}
}

func (server DnsServer) processRequest(writer dns.ResponseWriter, request *dns.Msg) error {
	response := new(dns.Msg)
	response.SetReply(request)

	if len(request.Question) != 1 {
		response.Rcode = dns.RcodeFormatError
		writer.WriteMsg(response)
		return writer.WriteMsg(response)
	}

	/* DNS RFC supports multiple questions, but in practise no DNS servers do. E.g. response status code NXDOMAIN
	 * does not make sense if there is more than one question, so in reality there is always only one. */
	question := request.Question[0]

	/* Check if the question is in our local cache, and if so, immediately return it. */
	records, exists, is_blocked := server.cache.GetRecord(question.Name, question.Qtype)
	if exists {
		if is_blocked {
			response.Rcode = dns.RcodeNameError
		} else {
			response.Answer = records
		}
		return writer.WriteMsg(response)
	}

	/* Check if question is in blacklist. */
	if server.filter.Matches(question.Name) {
		response.Rcode = dns.RcodeNameError
		server.cache.UpdateBlockedRecord(question.Name, question.Qtype)
		return writer.WriteMsg(response)
	}

	/* Forward unresolved question to another server */
	recursion_response, _, err := server.recurse(question)

	if err != nil && &recursion_response == nil {
		response.Rcode = dns.RcodeServerFailure
		return writer.WriteMsg(response)
	}

	if len(recursion_response.Answer) >= 1 {
		server.cache.UpdateRecord(question.Name, recursion_response.Answer)
		response.Answer = recursion_response.Answer
	}

	return writer.WriteMsg(response)
}
