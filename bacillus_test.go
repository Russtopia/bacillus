package main

// Trivial unit tests. TODO: make these meaningful.
import (
	"testing"
)

func Test1favIconHTML(t *testing.T) {
	if favIconHTML() != `<link rel="icon" type="image/jpg" href="/images/logo.jpg"/>` {
		t.Error("Unexpected HTML fragment")
	}
}

func Test2goBackJS(t *testing.T) {
	if goBackJS("2", "500") != `
<script>
  // Go back after a short delay
  setInterval(function(){ /*window.location.href = document.referrer;*/ window.history.go(-2); }, 500);
</script>
` {
		t.Error("Unexpected JS fragment")
	}
}

func Test3refreshMetaTag(t *testing.T) {
	if refreshMetaTag('r', "5") !=
		`<meta http-equiv="refresh" content="5">` {
		t.Error("Unexpected HTML fragment")
	}
}

func Test4refreshMetaTagInvalidRune(t *testing.T) {
	if refreshMetaTag('x', "5") != `` {
		t.Error("Unexpected HTML fragment")
	}
}
