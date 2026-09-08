package database

import (
	"encoding/json"
	"testing"
)

func TestHistoryBarsAndDeduplication(t *testing.T) {
	bars, err := ParseHistoryBars("265598", json.RawMessage(`{"barLength":60,"data":[{"t":1698796800000,"o":171.05,"h":192.93,"l":170.12,"c":189.37,"v":6942998.04},{"t":1698796800000,"o":170,"h":192,"l":169,"c":190}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if *bars[0].Volume != 6942998 || bars[0].TimestampMS != 1698796800000 || bars[0].Open != "171.05" || bars[1].Volume != nil {
		t.Fatal(bars)
	}
	unique := DeduplicateBars(bars)
	if len(unique) != 1 || unique[0].Close != "190" {
		t.Fatal(unique)
	}
	bars[1].BarLengthSeconds = 300
	if len(DeduplicateBars(bars)) != 2 {
		t.Fatal("different intervals collapsed")
	}
	for _, raw := range []string{`{"barLength":60}`, `{"barLength":60,"data":[{"t":1}]}`, `{"barLength":60,"data":[{"t":1,"o":"1"}]}`} {
		if _, err := ParseHistoryBars("1", json.RawMessage(raw)); err == nil {
			t.Fatal(raw)
		}
	}
}
