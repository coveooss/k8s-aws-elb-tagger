package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestAWSELBNameFromHostname(t *testing.T) {
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
			out, err := AWSELBNameFromHostname(tt.in)

			if (tt.shouldErr && err == nil) || (!tt.shouldErr && err != nil) || tt.out != out {
				t.Errorf("Got {\"%s\",%v}, want {\"%s\",%v}", out, err == nil, tt.out, tt.shouldErr)
			}
		})
	}
}
func TestAWSTagsFromK8SAnnotations(t *testing.T) {
	tagsToApplyFromAnnotationsTests := []struct {
		in  map[string]string
		out map[string]string
	}{
		{map[string]string{}, map[string]string{}},
		{map[string]string{"aws-tag/": "hello", "aws-tag-value/": "world", "aws-tag-key/": "!"}, map[string]string{}},
		{map[string]string{"aws-tag/owner": "John Doe"}, map[string]string{"owner": "John Doe"}},
		{map[string]string{"aws-tag-key/1": "owner", "aws-tag-value/1": "John Doe"}, map[string]string{"owner": "John Doe"}},
		{map[string]string{"aws-tag/owner": "John Doe", "aws-tag/greetings": "Hello World!", "aws-tag-key/1": "somewhere", "aws-tag-value/1": "in the house"}, map[string]string{"owner": "John Doe", "greetings": "Hello World!", "somewhere": "in the house"}},
	}

	for i, tt := range tagsToApplyFromAnnotationsTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			out := AWSTagsFromK8SAnnotations(tt.in)

			if !reflect.DeepEqual(tt.out, out) {
				t.Errorf("Got %v want %v", out, tt.out)
			}
		})
	}
}
