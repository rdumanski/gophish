package models

import "gopkg.in/check.v1"

func (s *ModelsSuite) TestDomainValidate(ch *check.C) {
	ok := Domain{Name: "PSE.org.pl", Role: "both"}
	ch.Assert(ok.Validate(), check.Equals, nil)
	ch.Assert(ok.Name, check.Equals, "pse.org.pl") // normalized lowercase

	ch.Assert((&Domain{Name: "", Role: "both"}).Validate(), check.Equals, ErrDomainNameNotSpecified)
	ch.Assert((&Domain{Name: "not a domain", Role: "both"}).Validate(), check.Equals, ErrDomainNameInvalid)
	ch.Assert((&Domain{Name: "http://x.com", Role: "both"}).Validate(), check.Equals, ErrDomainNameInvalid)
	ch.Assert((&Domain{Name: "x.com", Role: "bogus"}).Validate(), check.Equals, ErrDomainRoleInvalid)

	// empty role defaults to both
	d := Domain{Name: "x.com"}
	ch.Assert(d.Validate(), check.Equals, nil)
	ch.Assert(d.Role, check.Equals, DomainBoth)
}

func (s *ModelsSuite) TestDomainCRUDAndRecords(ch *check.C) {
	d := Domain{Name: "pse.org.pl", Role: DomainBoth, DKIMSelector: "s1"}
	ch.Assert(PostDomain(&d, 1), check.Equals, nil)
	ch.Assert(d.Id > 0, check.Equals, true)

	got, err := GetDomain(d.Id, 1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(got.Name, check.Equals, "pse.org.pl")
	// records attached on read: A + SPF + DKIM + DMARC for "both"
	ch.Assert(len(got.Records), check.Equals, 4)
	foundDKIMHost := false
	for _, r := range got.Records {
		if r.Host == "s1._domainkey.pse.org.pl" {
			foundDKIMHost = true
		}
	}
	ch.Assert(foundDKIMHost, check.Equals, true)
}

func (s *ModelsSuite) TestDomainRecordsByRole(ch *check.C) {
	landing := Domain{Name: "x.com", Role: DomainLanding}
	ch.Assert(len(landing.SuggestedRecords()), check.Equals, 1) // just the A/CNAME
	sending := Domain{Name: "x.com", Role: DomainSending}
	ch.Assert(len(sending.SuggestedRecords()), check.Equals, 3) // SPF+DKIM+DMARC
}

func (s *ModelsSuite) TestDomainHealthParsers(ch *check.C) {
	ch.Assert(SPFPresent([]string{"some=thing", "v=spf1 include:_spf.x.com ~all"}), check.Equals, true)
	ch.Assert(SPFPresent([]string{"v=DMARC1; p=none"}), check.Equals, false)
	ch.Assert(DMARCPresent([]string{"v=DMARC1; p=reject"}), check.Equals, true)
	ch.Assert(DKIMPresent([]string{"v=DKIM1; k=rsa; p=MIGfMA0..."}), check.Equals, true)
	ch.Assert(DKIMPresent([]string{"v=DKIM1; k=rsa;"}), check.Equals, false) // no p=
	ch.Assert(RobotsMarkerOK("User-agent: *\nDisallow: /\n"), check.Equals, true)
	ch.Assert(RobotsMarkerOK("<html>not gophish</html>"), check.Equals, false)
}
