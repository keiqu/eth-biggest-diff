package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccounts(t *testing.T) {
	file, err := os.ReadFile("testdata.json")
	if err != nil {
		t.Fatal(err)
	}

	var blockInfo BlockInfo
	err = json.Unmarshal(file, &blockInfo)
	if err != nil {
		t.Fatal(err)
	}

	a := accounts{}
	a.update(blockInfo.Transactions)
	addr, value := a.getMax()
	assert.Equal(t, addr, "0x49b21bdfa30333858956342f4028ce72e37eb851")
	assert.Equal(t, value, "-460110000000000000")
}
