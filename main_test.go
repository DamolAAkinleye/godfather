package main

import (
	"testing"
)

func TestHandler(t *testing.T) {
	input := LambdaRule{
		Zone:   "in.brcclx.com.",
		Master: "10.11.100.30",
		ZoneID: "ZWZN7TVAM3N8V",
	}

	HandleRequest(input)
}
