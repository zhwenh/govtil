package jail

import (
	"testing"
)

func TestFaked(t *testing.T) {
	killch, _, err := faked("", "")
	if err != nil {
		t.Error(err)
	} else {
		killch <- true
		<-killch // confirm done
	}
}

// TODO(vsekhar): test NewChrootJail()
