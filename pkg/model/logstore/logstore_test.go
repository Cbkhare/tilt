package logstore

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/pkg/model"
)

func TestLog_AppendUnderLimit(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("foo"), nil)
	l.Append(newGlobalTestLogEvent("bar"), nil)
	assert.Equal(t, "foobar", l.String())
}

func TestLog_AppendOverLimit(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("hello\n"), nil)
	sb := strings.Builder{}
	for i := 0; i < maxLogLengthInBytes/2; i++ {
		_, err := sb.WriteString("x\n")
		if err != nil {
			t.Fatalf("error in %T.WriteString: %+v", sb, err)
		}
	}

	s := sb.String()
	l.Append(newGlobalTestLogEvent(s), nil)
	assert.Equal(t, s[:logTruncationTarget], l.String())
}

func TestLogPrefix(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("hello\n"), nil)
	l.Append(newTestLogEvent("prefix", time.Now(), "bar\nbaz\n"), nil)
	expected := "hello\nprefix      ┊ bar\nprefix      ┊ baz\n"
	assert.Equal(t, expected, l.String())
}

// Assert that when logs come from two different sources, they get interleaved correctly.
func TestLogInterleaving(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("hello ... "), nil)
	l.Append(newTestLogEvent("prefix", time.Now(), "START LONG MESSAGE\ngoodbye ... "), nil)
	l.Append(newGlobalTestLogEvent("world\nnext line of global log"), nil)
	l.Append(newTestLogEvent("prefix", time.Now(), "world\nEND LONG MESSAGE"), nil)
	expected := "hello ... world\nprefix      ┊ START LONG MESSAGE\nprefix      ┊ goodbye ... world\nnext line of global log\nprefix      ┊ END LONG MESSAGE"
	assert.Equal(t, expected, l.String())
}

func TestScrubSecret(t *testing.T) {
	l := NewLogStore()
	secretSet := model.SecretSet{}
	secretSet.AddSecret("my-secret", "client-id", []byte("secret"))
	l.Append(newGlobalTestLogEvent("hello\nsecret-time!\nc2VjcmV0-time!\ngoodbye"), secretSet)
	assert.Equal(t, `hello
[redacted secret my-secret:client-id]-time!
[redacted secret my-secret:client-id]-time!
goodbye`, l.String())
}

func TestLogTail(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("1\n2\n3\n4\n5\n"), nil)
	assert.Equal(t, "", l.Tail(0).String())
	assert.Equal(t, "5\n", l.Tail(1).String())
	assert.Equal(t, "4\n5\n", l.Tail(2).String())
	assert.Equal(t, "3\n4\n5\n", l.Tail(3).String())
	assert.Equal(t, "2\n3\n4\n5\n", l.Tail(4).String())
	assert.Equal(t, "1\n2\n3\n4\n5\n", l.Tail(5).String())
	assert.Equal(t, "1\n2\n3\n4\n5\n", l.Tail(6).String())
}

func TestLogTailPrefixes(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("1\n2\n"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "3\n4\n"), nil)
	l.Append(newGlobalTestLogEvent("5\n"), nil)
	assert.Equal(t, "", l.Tail(0).String())
	assert.Equal(t, "5\n", l.Tail(1).String())
	assert.Equal(t, "fe          ┊ 4\n5\n", l.Tail(2).String())
	assert.Equal(t, "fe          ┊ 3\nfe          ┊ 4\n5\n", l.Tail(3).String())
	assert.Equal(t, "2\nfe          ┊ 3\nfe          ┊ 4\n5\n", l.Tail(4).String())
	assert.Equal(t, "1\n2\nfe          ┊ 3\nfe          ┊ 4\n5\n", l.Tail(5).String())
	assert.Equal(t, "1\n2\nfe          ┊ 3\nfe          ┊ 4\n5\n", l.Tail(6).String())
}

func TestLogTailParts(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("a"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "xy"), nil)
	l.Append(newGlobalTestLogEvent("bc\n"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "z\n"), nil)
	assert.Equal(t, "fe          ┊ xyz\n", l.Tail(1).String())
	assert.Equal(t, "abc\nfe          ┊ xyz\n", l.Tail(2).String())
}

func TestContinuingString(t *testing.T) {
	l := NewLogStore()

	c1 := l.Checkpoint()
	assert.Equal(t, "", l.ContinuingString(c1))

	l.Append(newGlobalTestLogEvent("foo"), nil)
	c2 := l.Checkpoint()
	assert.Equal(t, "foo", l.ContinuingString(c1))

	l.Append(newGlobalTestLogEvent("bar\n"), nil)
	assert.Equal(t, "foobar\n", l.ContinuingString(c1))
	assert.Equal(t, "bar\n", l.ContinuingString(c2))
}

func TestContinuingStringOneSource(t *testing.T) {
	l := NewLogStore()

	c1 := l.Checkpoint()
	assert.Equal(t, "", l.ContinuingString(c1))

	l.Append(newTestLogEvent("fe", time.Now(), "foo"), nil)
	c2 := l.Checkpoint()
	assert.Equal(t, "fe          ┊ foo", l.ContinuingString(c1))

	l.Append(newTestLogEvent("fe", time.Now(), "bar\n"), nil)
	assert.Equal(t, "fe          ┊ foobar\n", l.ContinuingString(c1))
	assert.Equal(t, "bar\n", l.ContinuingString(c2))
}

func TestContinuingStringTwoSources(t *testing.T) {
	l := NewLogStore()

	c1 := l.Checkpoint()

	l.Append(newGlobalTestLogEvent("a"), nil)
	c2 := l.Checkpoint()
	assert.Equal(t, "a", l.ContinuingString(c1))

	l.Append(newTestLogEvent("fe", time.Now(), "xy"), nil)
	c3 := l.Checkpoint()
	assert.Equal(t, "a\nfe          ┊ xy", l.ContinuingString(c1))
	assert.Equal(t, "\nfe          ┊ xy", l.ContinuingString(c2))

	l.Append(newGlobalTestLogEvent("bc\n"), nil)
	c4 := l.Checkpoint()
	assert.Equal(t, "abc\nfe          ┊ xy", l.ContinuingString(c1))
	assert.Equal(t, "\nfe          ┊ xy\nbc\n", l.ContinuingString(c2))
	assert.Equal(t, "\nbc\n", l.ContinuingString(c3))

	l.Append(newTestLogEvent("fe", time.Now(), "z\n"), nil)
	assert.Equal(t, "abc\nfe          ┊ xyz\n", l.ContinuingString(c1))
	assert.Equal(t, "\nfe          ┊ xyz\nbc\n", l.ContinuingString(c2))
	assert.Equal(t, "\nbc\nfe          ┊ z\n", l.ContinuingString(c3))
	assert.Equal(t, "fe          ┊ z\n", l.ContinuingString(c4))
}