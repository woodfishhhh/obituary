package ledger

import (
	"testing"
	"time"
)

func TestAutoReceiptsFollowTrackedItems(t *testing.T) {
	st := State{Items: []Item{{ID: 1}, {ID: 2}}}
	now := time.Now()
	st.AddAutoReceipts(AutoReceipt{ID: 1, Class: AutoCreated, At: now}, AutoReceipt{ID: 2, Class: AutoContainer, At: now}, AutoReceipt{ID: 9, Class: AutoAll})
	if len(st.AutoReceipts) != 2 {
		t.Fatalf("receipt without an item kept: %+v", st.AutoReceipts)
	}
	st.Items = st.Items[1:]
	handled := st.AutoHandled()
	if len(handled) != 1 || handled[0].Item.ID != 2 || handled[0].Class != AutoContainer {
		t.Fatalf("handled = %+v", handled)
	}
	if _, ok := st.AutoReceipt(1); !ok {
		t.Fatal("receipts are pruned on the next write, not on read")
	}
	for i := int64(0); i < maxAutoReceipts+5; i++ {
		st.Items = append(st.Items, Item{ID: 100 + i})
		st.AddAutoReceipts(AutoReceipt{ID: 100 + i})
	}
	if len(st.AutoReceipts) != maxAutoReceipts || st.AutoReceipts[len(st.AutoReceipts)-1].ID != 100+maxAutoReceipts+4 {
		t.Fatalf("bound kept %d receipts", len(st.AutoReceipts))
	}
}
