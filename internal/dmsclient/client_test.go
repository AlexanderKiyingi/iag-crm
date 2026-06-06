package dmsclient

import (
	"encoding/json"
	"testing"
)

func TestListOutletsParsesItemsKey(t *testing.T) {
	raw := `{"items":[{"id":"OUT-1","name":"Test"}],"meta":{"total":1}}`
	var wrapped struct {
		Items []Outlet `json:"items"`
		Data  []Outlet `json:"data"`
		Meta  struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err != nil {
		t.Fatal(err)
	}
	items := wrapped.Items
	if len(items) != 1 || items[0].ID != "OUT-1" {
		t.Fatalf("got %+v", items)
	}
}
