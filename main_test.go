package main

import (
	"testing"
)

func TestHandler(t *testing.T) {
	input := LambdaRule{
		Zone:   "in.creditcards.com.",
		Master: "10.11.100.30",
		ZoneID: "Z2PCL613VMNHI5",
	}

	HandleRequest(input)
}
