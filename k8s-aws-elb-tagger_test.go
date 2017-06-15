package main

import (
	"fmt"
	"testing"
)

func TestLoadBalancerNameFromHostname(t *testing.T) {
	loadBalancerNameFromHostnameTests := []struct {
		in        string
		out       string
		shouldErr bool
	}{
		{"", "", true},
		{"testpublic-1111111111.us-east-1.elb.amazonaws.com", "testpublic", false},
		{"internal-testinternal-2222222222.us-east-1.elb.amazonaws.com", "testinternal", false},
		{"ac6fa59d9425c11e785f90e87cb400be-6752909.us-east-1.elb.amazonaws.com", "ac6fa59d9425c11e785f90e87cb400be", false},
		{"invalid", "", true},
	}

	for _, tt := range loadBalancerNameFromHostnameTests {
		name := fmt.Sprintf("\"%s\"", tt.in)
		t.Run(name, func(t *testing.T) {
			out, err := LoadBalancerNameFromHostname(tt.in)

			if (tt.shouldErr && err == nil) || (!tt.shouldErr && err != nil) || tt.out != out {
				t.Errorf("Got {\"%s\",%v}, want {\"%s\",%v}", out, err == nil, tt.out, tt.shouldErr)
			}
		})
	}
}
