package models

import (
	"bufio"
	"encoding/csv"
	"io"
	"net/mail"
	"regexp"
)

// Column-header matchers for recipient CSVs. Matched case-insensitively and
// anywhere in the header cell, so column order and extra columns don't matter.
var (
	csvFirstNameRegex = regexp.MustCompile(`(?i)first[\s_-]*name`)
	csvLastNameRegex  = regexp.MustCompile(`(?i)last[\s_-]*name`)
	csvEmailRegex     = regexp.MustCompile(`(?i)email`)
	csvPositionRegex  = regexp.MustCompile(`(?i)position`)
)

// ParseCSVReader parses a CSV stream of recipients into Targets. It is the
// shared core behind util.ParseCSV (HTTP upload) and the email-to-mailbox
// roster sync, so header detection lives in exactly one place. A leading UTF-8
// BOM (common in Excel/Outlook exports) is stripped so it can't corrupt the
// first header; ragged rows and rows with an unparseable email are skipped
// rather than aborting the whole import.
func ParseCSVReader(r io.Reader) ([]Target, error) {
	ts := []Target{}
	reader := csv.NewReader(stripBOM(r))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err == io.EOF {
		return ts, nil
	}
	if err != nil {
		return ts, err
	}
	fi, li, ei, pi := -1, -1, -1, -1
	for i, v := range header {
		switch {
		case csvFirstNameRegex.MatchString(v):
			fi = i
		case csvLastNameRegex.MatchString(v):
			li = i
		case csvEmailRegex.MatchString(v):
			ei = i
		case csvPositionRegex.MatchString(v):
			pi = i
		}
	}
	if fi == -1 && li == -1 && ei == -1 && pi == -1 {
		return ts, nil
	}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		fn, ln, ea, ps := "", "", "", ""
		if fi != -1 && len(record) > fi {
			fn = record[fi]
		}
		if li != -1 && len(record) > li {
			ln = record[li]
		}
		if ei != -1 && len(record) > ei {
			csvEmail, err := mail.ParseAddress(record[ei])
			if err != nil {
				continue
			}
			ea = csvEmail.Address
		}
		if pi != -1 && len(record) > pi {
			ps = record[pi]
		}
		ts = append(ts, Target{
			BaseRecipient: BaseRecipient{FirstName: fn, LastName: ln, Email: ea, Position: ps},
		})
	}
	return ts, nil
}

// stripBOM returns a reader with a leading UTF-8 byte-order mark removed.
func stripBOM(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	bom, err := br.Peek(3)
	if err == nil && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		_, _ = br.Discard(3)
	}
	return br
}
