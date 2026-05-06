package gomail

import (
	"bytes"
	"io"
	"net/textproto"
	"reflect"
	"testing"
)

const (
	testTo1  = "to1@example.com"
	testTo2  = "to2@example.com"
	testFrom = "from@example.com"
	testAlternativeFrom = "from@evildomain.com"
	testBody = "Test message"
	testMsg  = "To: " + testTo1 + ", " + testTo2 + "\r\n" +
		"From: " + testFrom + "\r\n" +
		"Mime-Version: 1.0\r\n" +
		"Date: Wed, 25 Jun 2014 17:46:00 +0000\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		testBody
)

type mockSender SendFunc

func (s mockSender) Send(from string, to []string, msg io.WriterTo) error {
	return s(from, to, msg)
}

type mockSendCloser struct {
	mockSender
	close func() error
	reset func() error
}

func (s *mockSendCloser) Close() error {
	return s.close()
}

func (s *mockSendCloser) Reset() error {
	return s.reset()
}

func TestSendSpoof(t *testing.T) {
	s := &mockSendCloser{
		mockSender: stubSend(t, testAlternativeFrom, []string{testTo1, testTo2}, testMsg),
		close: func() error {
			t.Error("Close() should not be called in Send()")
			return nil
		},
		reset: func() error {
			t.Error("Reset() should not be called in Send()")
			return nil
		},
	}
	if err := SendCustomFrom(s, testAlternativeFrom, getTestMessage()); err != nil {
		t.Errorf("Send(): %v", err)
	}
}


func TestSend(t *testing.T) {
	s := &mockSendCloser{
		mockSender: stubSend(t, testFrom, []string{testTo1, testTo2}, testMsg),
		close: func() error {
			t.Error("Close() should not be called in Send()")
			return nil
		},
		reset: func() error {
			t.Error("Reset() should not be called in Send()")
			return nil
		},
	}
	if err := Send(s, getTestMessage()); err != nil {
		t.Errorf("Send(): %v", err)
	}
}

func TestSendError(t *testing.T) {
	expected := &textproto.Error{
		Code: 400,
		Msg:  "Error",
	}

	s := &mockSendCloser{
		mockSender: func(from string, to []string, msg io.WriterTo) error {
			return expected
		},
	}
	err := Send(s, getTestMessage())
	if err == nil {
		t.Errorf("Expected error in Send(): %v", err)
	}
	if got, ok := err.(*textproto.Error); ok {
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("Invalid error received in Send(). Expected %v Got %v", expected, got)
		}
	} else {
		t.Errorf("Invalid error type in Send(): %v", err)
	}
}

func getTestMessage() *Message {
	m := NewMessage()
	m.SetHeader("From", testFrom)
	m.SetHeader("To", testTo1, testTo2)
	m.SetBody("text/plain", testBody)

	return m
}

func stubSend(t *testing.T, wantFrom string, wantTo []string, wantBody string) mockSender {
	return func(from string, to []string, msg io.WriterTo) error {
		if from != wantFrom {
			t.Errorf("invalid from, got %q, want %q", from, wantFrom)
		}
		if !reflect.DeepEqual(to, wantTo) {
			t.Errorf("invalid to, got %v, want %v", to, wantTo)
		}

		buf := new(bytes.Buffer)
		_, err := msg.WriteTo(buf)
		if err != nil {
			t.Fatal(err)
		}
		compareBodies(t, buf.String(), wantBody)

		return nil
	}
}
