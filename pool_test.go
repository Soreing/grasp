package grasp

import (
	"context"
	"testing"
	"time"
)

type Item struct {
	count int
	done  bool
}

func NewItem(count int) *Item {
	return &Item{count, false}
}

func (i *Item) PoolRelease() {
	i.done = true
}

func TestServe_Empty(t *testing.T) {
	count := 0
	pl := NewPool(5, time.Second, func() Poolable {
		count++
		return NewItem(count)
	}).(*pool)

	val1, done1, err1 := pl.Acquire(context.Background())
	val2, done2, err2 := pl.Acquire(context.Background())
	val3, done3, err3 := pl.Acquire(context.Background())

	if err1 != nil || err2 != nil || err3 != nil {
		t.Errorf("no error expected")
	} else if val1 == nil || done1 == nil ||
		val2 == nil || done2 == nil ||
		val3 == nil || done3 == nil {
		t.Errorf("values not expected to be nil")
	} else {
		it1, ok1 := val1.(*Item)
		it2, ok2 := val2.(*Item)
		it3, ok3 := val3.(*Item)
		if !ok1 || !ok2 || !ok3 {
			t.Errorf("value type is not Item ptr")
		} else if it1.count != 1 || it2.count != 2 || it3.count != 3 {
			t.Errorf("item count is wrong")
		} else if pl.count != 3 {
			t.Errorf("pool size epected to be %d but it's %d", count, pl.count)
		}
	}
}

func TestServe_Idle(t *testing.T) {
	pl := NewPool(5, time.Second, func() Poolable {
		return NewItem(0)
	}).(*pool)

	pl.count += 3
	pl.exch <- &resource{value: NewItem(1)}
	pl.exch <- &resource{value: NewItem(2)}
	pl.exch <- &resource{value: NewItem(3)}
	time.Sleep(time.Millisecond * 100)

	val1, done1, err1 := pl.Acquire(context.Background())
	val2, done2, err2 := pl.Acquire(context.Background())

	if err1 != nil || err2 != nil {
		t.Errorf("no error expected")
	} else if val1 == nil || done1 == nil ||
		val2 == nil || done2 == nil {
		t.Errorf("values not expected to be nil")
	} else {
		it1, ok1 := val1.(*Item)
		it2, ok2 := val2.(*Item)
		if !ok1 || !ok2 {
			t.Errorf("value type is not Item ptr")
		} else if it1.count == 0 || it2.count == 0 || it1.count == it2.count {
			t.Errorf("item count is wrong")
		} else if pl.count != 3 {
			t.Errorf("pool size epected to be %d but it's %d", 3, pl.count)
		} else if pl.icnt != 1 || len(pl.idle) != 1 {
			t.Errorf("idle size epected to be %d but it's %d", 1, pl.icnt)
		}
	}
}

func TestServe_Full(t *testing.T) {
	pl := NewPool(3, time.Second, func() Poolable {
		return NewItem(0)
	}).(*pool)

	pl.count += 3
	done := false

	go func() {
		pl.Acquire(context.Background())
		done = true
	}()

	time.Sleep(time.Millisecond * 100)

	if done {
		t.Errorf("call was not expected to succeed")
	} else if len(pl.pend) != 1 {
		t.Errorf("pending requests are %d, expected %d", len(pl.pend), 1)
	}
}

func TestRelease(t *testing.T) {
	pl := NewPool(3, time.Second, func() Poolable {
		return NewItem(0)
	}).(*pool)

	val1, done1, err1 := pl.Acquire(context.Background())
	val2, done2, err2 := pl.Acquire(context.Background())
	val3, done3, err3 := pl.Acquire(context.Background())

	if err1 != nil || err2 != nil || err3 != nil {
		t.Errorf("no error expected")
	} else if val1 == nil || done1 == nil ||
		val2 == nil || done2 == nil ||
		val3 == nil || done3 == nil {
		t.Errorf("values not expected to be nil")
	} else {
		done1()
		done2()
		done3()
		time.Sleep(time.Millisecond * 100)
		if pl.icnt != 3 || len(pl.idle) != 3 {
			t.Errorf("expected idle resources to be %d", 3)
		}
	}
}

func TestExpire(t *testing.T) {
	pl := NewPool(3, time.Millisecond*100, func() Poolable {
		return NewItem(0)
	}).(*pool)

	val1, done1, err1 := pl.Acquire(context.Background())
	val2, done2, err2 := pl.Acquire(context.Background())
	val3, done3, err3 := pl.Acquire(context.Background())

	if err1 != nil || err2 != nil || err3 != nil {
		t.Errorf("no error expected")
	} else if val1 == nil || done1 == nil ||
		val2 == nil || done2 == nil ||
		val3 == nil || done3 == nil {
		t.Errorf("values not expected to be nil")
	} else {
		it1, ok1 := val1.(*Item)
		it2, ok2 := val2.(*Item)
		it3, ok3 := val3.(*Item)
		if !ok1 || !ok2 || !ok3 {
			t.Errorf("value type is not Item ptr")
		} else {
			done1()
			done2()
			done3()
			time.Sleep(time.Millisecond * 200)
			if !it1.done || !it2.done || !it3.done {
				t.Errorf("some items are not released")
			} else if pl.icnt != 0 || len(pl.idle) != 0 {
				t.Errorf("expected idle resources to be %d", 3)
			} else if pl.count != 0 {
				t.Errorf("expected pool size to be 0")
			}
		}
	}
}
