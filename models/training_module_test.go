package models

import (
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestPostTrainingModuleHTML(ch *check.C) {
	m := TrainingModule{Name: "Spotting Phishing", ContentType: TrainingContentHTML, HTML: "<h1>Hi</h1>", UserID: 1}
	ch.Assert(PostTrainingModule(&m), check.Equals, nil)
	ch.Assert(m.Id > 0, check.Equals, true)

	got, err := GetTrainingModule(m.Id, 1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(got.Name, check.Equals, "Spotting Phishing")
	ch.Assert(got.ContentType, check.Equals, TrainingContentHTML)
	ch.Assert(got.HTML, check.Equals, "<h1>Hi</h1>")
}

func (s *ModelsSuite) TestTrainingModuleValidation(ch *check.C) {
	cases := []struct {
		m   TrainingModule
		err error
	}{
		{TrainingModule{ContentType: TrainingContentHTML, HTML: "x"}, ErrTrainingNameNotSpecified},
		{TrainingModule{Name: "n", ContentType: "bogus"}, ErrTrainingUnknownContentType},
		{TrainingModule{Name: "n", ContentType: TrainingContentHTML}, ErrTrainingMissingHTML},
		{TrainingModule{Name: "n", ContentType: TrainingContentVideo}, ErrTrainingMissingURL},
		{TrainingModule{Name: "n", ContentType: TrainingContentExternal, URL: "not-a-url"}, ErrTrainingInvalidURL},
		{TrainingModule{Name: "n", ContentType: TrainingContentExternal, URL: "ftp://x.com/a"}, ErrTrainingInvalidURL},
	}
	for _, c := range cases {
		ch.Assert(c.m.Validate(), check.Equals, c.err)
	}

	// Valid video module passes.
	ok := TrainingModule{Name: "v", ContentType: TrainingContentVideo, URL: "https://example.com/v"}
	ch.Assert(ok.Validate(), check.Equals, nil)
}

func (s *ModelsSuite) TestPutAndDeleteTrainingModule(ch *check.C) {
	m := TrainingModule{Name: "Mod", ContentType: TrainingContentExternal, URL: "https://example.com/a", UserID: 1}
	ch.Assert(PostTrainingModule(&m), check.Equals, nil)

	m.Name = "Mod Renamed"
	m.URL = "https://example.com/b"
	ch.Assert(PutTrainingModule(&m), check.Equals, nil)

	got, err := GetTrainingModule(m.Id, 1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(got.Name, check.Equals, "Mod Renamed")
	ch.Assert(got.URL, check.Equals, "https://example.com/b")

	ch.Assert(DeleteTrainingModule(m.Id, 1), check.Equals, nil)
	_, err = GetTrainingModule(m.Id, 1)
	ch.Assert(err, check.NotNil)
}
