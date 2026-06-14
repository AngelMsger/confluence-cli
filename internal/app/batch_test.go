package app

import (
	"reflect"
	"strings"
	"testing"
)

func TestCollectBatchArgs(t *testing.T) {
	t.Run("positional args pass through", func(t *testing.T) {
		got, err := collectBatchArgs([]string{"1", "2", "3"}, strings.NewReader("ignored"))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []string{"1", "2", "3"}) {
			t.Errorf("got %v", got)
		}
	})

	t.Run("dash reads stdin, trimming blanks", func(t *testing.T) {
		got, err := collectBatchArgs([]string{"-"}, strings.NewReader("  10 \n\n20\n\t30\t\n"))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []string{"10", "20", "30"}) {
			t.Errorf("got %v", got)
		}
	})

	t.Run("empty stdin is an error", func(t *testing.T) {
		_, err := collectBatchArgs([]string{"-"}, strings.NewReader("\n  \n"))
		if err == nil || !strings.Contains(err.Error(), "no items") {
			t.Errorf("want NO_ITEMS error, got %v", err)
		}
	})
}
