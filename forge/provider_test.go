package forge

import (
	"strings"
	"testing"
)

func TestCheckoutRefusesEmbeddedCredentialsAndForeignHosts(t *testing.T) {
	valid := Checkout{HTTPS: "https://forge.example/o/r.git", SSH: "ssh://git@forge.example:2222/o/r.git", HeadRef: "refs/pull/1/head"}
	if err := ValidateCheckout(valid, "forge.example"); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*Checkout){
		func(c *Checkout) { c.HTTPS = "https://user:secret@forge.example/o/r.git" },
		func(c *Checkout) { c.HTTPS = "https://elsewhere.example/o/r.git" },
		func(c *Checkout) { c.HTTPS += "?token=secret" },
		func(c *Checkout) { c.SSH = "ssh://git:secret@forge.example/o/r.git" },
		func(c *Checkout) { c.SSH = "ssh://git@elsewhere.example/o/r.git" },
		func(c *Checkout) { c.SSH = "ext::arbitrary command" },
		func(c *Checkout) { c.HeadRef = "refs/pull/1/head:refs/heads/main" },
		func(c *Checkout) { c.HeadRef = "refs/pull/*/head" },
		func(c *Checkout) { c.HeadRef = "-refs/pull/1/head" },
	} {
		c := valid
		edit(&c)
		err := ValidateCheckout(c, "forge.example")
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("unsafe metadata accepted or exposed")
		}
	}
}
