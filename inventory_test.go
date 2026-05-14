package main

import (
	"reflect"
	"testing"
)

func TestNormalizePots_mergeAndSort(t *testing.T) {
	got, err := normalizePots([]PotStack{
		{ItemID: "pot_gold", Quantity: 1},
		{ItemID: "pot_wood", Quantity: 2},
		{ItemID: "pot_wood", Quantity: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []PotStack{
		{ItemID: "pot_gold", Quantity: 1},
		{ItemID: "pot_wood", Quantity: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestConsumeOnePot(t *testing.T) {
	inv := PlayerInventory{Pots: []PotStack{
		{ItemID: "pot_wood", Quantity: 2},
		{ItemID: "pot_gold", Quantity: 1},
	}}
	if err := ConsumeOnePot(&inv, "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeOnePot(&inv, "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeOnePot(&inv, "pot_wood"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizePots_dropInvalid(t *testing.T) {
	got, err := normalizePots([]PotStack{
		{ItemID: "", Quantity: 1},
		{ItemID: "pot_wood", Quantity: 0},
		{ItemID: "  x  ", Quantity: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []PotStack{{ItemID: "x", Quantity: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
