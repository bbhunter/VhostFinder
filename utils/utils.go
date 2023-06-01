package utils

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type Options struct {
	Domains  []string
	Headers  []string
	Ips      []string
	Paths    []string
	Port     int
	Proxy    string
	Threads  int
	Timeout  int
	Tls      bool
	Verbose  bool
	Verify   bool
	Wordlist []string
}

type Job struct {
	Baseline *FuzzResult
	Domain   string
	Ip       string
	Path     string
}

func EnumerateVhosts(opts *Options) {
	domains := PermuteDomains(opts.Wordlist, opts.Domains)

	threadChan := make(chan Job, opts.Threads)
	var wg sync.WaitGroup

	fuzzer := &Fuzzer{
		Options: opts,
		Client:  getClient(opts),
	}

	for i := 0; i < cap(threadChan); i++ {
		go worker(fuzzer, threadChan, &wg)
	}

	for _, ip := range opts.Ips {
		for _, path := range opts.Paths {
			baseUrl := fuzzer.GetBaseUrl(ip, path)
			if opts.Verbose {
				fmt.Printf("[!] Obtaining baseline on: %s\n", baseUrl)
			}
			baseline, err := fuzzer.FuzzHost(ip, uuid.NewString(), path)
			if err != nil {
				fmt.Printf("[!] Failed to obtain baseline (%s): %s\n", baseUrl, err.Error())
			} else {
				for _, domain := range domains {
					wg.Add(1)
					threadChan <- Job{
						Baseline: baseline,
						Domain:   domain,
						Ip:       ip,
						Path:     path,
					}
				}
			}
		}
	}
	wg.Wait()
	close(threadChan)
}

func worker(f *Fuzzer, jobs chan Job, wg *sync.WaitGroup) {
	for job := range jobs {
		result, resp, err := f.TestDomain(job.Ip, job.Domain, job.Path, job.Baseline.Response)
		if err != nil {
			fmt.Printf("[!] [%s] [%s] [%d] [%d] %s -> %s\n", job.Ip, job.Path, job.Domain, resp.Status, resp.ContentLength, err.Error())
		} else if result == true {
			if f.Options.Verify {
				if f.CompareGeneric(job.Domain, job.Path, resp.Response) {
					fmt.Printf("[+] [%s] [%s] [%d] [%d] %s\n", job.Ip, job.Path, resp.Status, resp.ContentLength, job.Domain)
				} else {
					fmt.Printf("[-] [%s] [%s] [%d] [%d] %s is different than the baseline, but is not different than public facing domain\n", job.Ip, job.Path, resp.Status, resp.ContentLength, job.Domain)
				}
			} else {
				fmt.Printf("[+] [%s] [%s] [%d] [%d] %s\n", job.Ip, job.Path, resp.Status, resp.ContentLength, job.Domain)
			}
		} else if f.Options.Verbose {
			fmt.Printf("[-] [%s] [%s] [%d] [%d] %s is not different than the baseline\n", job.Ip, job.Path, resp.Status, resp.ContentLength, job.Domain)
		}
		wg.Done()
	}
}

func PermuteDomains(wordlist []string, domainList []string) []string {
	var domains []string
	var dnSet bool = len(domainList) > 0

	for _, guess := range wordlist {
		if dnSet {
			for _, domain := range domainList {
				domains = append(domains, fmt.Sprintf("%s.%s", guess, domain))
			}
		} else {
			domains = append(domains, guess)
		}
	}

	return domains
}
