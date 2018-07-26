package agi

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// State describes the Asterisk channel state.  There are mapped
// directly to the Asterisk enumerations.
type State int

const (
	// StateDown indicates the channel is down and available
	StateDown State = iota

	// StateReserved indicates the channel is down but reserved
	StateReserved

	// StateOffhook indicates that the channel is offhook
	StateOffhook

	// StateDialing indicates that digits have been dialed
	StateDialing

	// StateRing indicates the channel is ringing
	StateRing

	// StateRinging indicates the channel's remote end is rining (the channel is receiving ringback)
	StateRinging

	// StateUp indicates the channel is up
	StateUp

	// StateBusy indicates the line is busy
	StateBusy

	// StateDialingOffHook indicates digits have been dialed while offhook
	StateDialingOffHook

	// StatePreRing indicates the channel has detected an incoming call and is waiting for ring
	StatePreRing
)

// AGI represents an AGI session
type AGI struct {
	// Variables stored the initial variables
	// transmitted from Asterisk at the start
	// of the AGI session.
	Variables map[string]string

	r    io.Reader
	eagi io.Reader
	w    io.Writer

	conn net.Conn

	mu sync.Mutex
}

// Response represents a response to an AGI
// request.
type Response struct {
	Error        error  // Error received, if any
	Status       int    // HTTP-style status code received
	Result       int    // Result is the numerical return (if parseable)
	ResultString string // Result value as a string
	Value        string // Value is the (optional) string value returned
}

// Err returns the error value from the response
func (r *Response) Err() error {
	return r.Error
}

// Val returns the response value and error
func (r *Response) Val() (string, error) {
	return r.Value, r.Error
}

// Regex for AGI response result code and value
var responseRegex = regexp.MustCompile(`^([\d]{3})\sresult=(\-?[[:alnum:]]*)(\s.*)?$`)

const (
	// StatusOK indicates the AGI command was
	// accepted.
	StatusOK = 200

	// StatusInvalid indicates Asterisk did not
	// understand the command.
	StatusInvalid = 510

	// StatusDeadChannel indicates that the command
	// cannot be performed on a dead (hungup) channel.
	StatusDeadChannel = 511

	// StatusEndUsage indicates...TODO
	StatusEndUsage = 520
)

// HandlerFunc is a function which accepts an AGI instance
type HandlerFunc func(*AGI)

// New creates an AGI session from the given reader and writer.
func New(r io.Reader, w io.Writer) *AGI {
	return NewWithEAGI(r, w, nil)
}

// NewWithEAGI returns a new AGI session to the given `os.Stdin` `io.Reader`,
// EAGI `io.Reader`, and `os.Stdout` `io.Writer`. The initial variables will
// be read in.
func NewWithEAGI(r io.Reader, w io.Writer, eagi io.Reader) *AGI {
	a := AGI{
		Variables: make(map[string]string),
		r:         r,
		w:         w,
		eagi:      eagi,
	}

	s := bufio.NewScanner(a.r)
	for s.Scan() {
		if s.Text() == "" {
			break
		}

		terms := strings.Split(s.Text(), ":")
		if len(terms) == 2 {
			a.Variables[strings.TrimSpace(terms[0])] = strings.TrimSpace(terms[1])
		}
	}

	return &a
}

// NewConn returns a new AGI session bound to the given net.Conn interface
func NewConn(conn net.Conn) *AGI {
	a := New(conn, conn)
	a.conn = conn
	return a
}

// NewStdio returns a new AGI session to stdin and stdout.
func NewStdio() *AGI {
	return New(os.Stdin, os.Stdout)
}

// NewEAGI returns a new AGI session to stdin, the EAGI stream (FD=3), and stdout.
func NewEAGI() *AGI {
	return NewWithEAGI(os.Stdin, os.Stdout, os.NewFile(uintptr(3), "/dev/stdeagi"))
}

// Listen binds an AGI HandlerFunc to the given TCP `host:port` address, creating a FastAGI service.
func Listen(addr string, handler HandlerFunc) error {
	if addr == "" {
		addr = "localhost:4573"
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "failed to bind server")
	}
	defer l.Close() // nolint: errcheck

	for {
		conn, err := l.Accept()
		if err != nil {
			return errors.Wrap(err, "failed to accept TCP connection")
		}

		go handler(NewConn(conn))
	}
}

// Close closes any network connection associated with the AGI instance
func (a *AGI) Close() (err error) {
	if a.conn != nil {
		err = a.conn.Close()
		a.conn = nil
	}
	return
}

// EAGI enables access to the EAGI incoming stream (if available).
func (a *AGI) EAGI() io.Reader {
	return a.eagi
}

// Command sends the given command line to stdout
// and returns the response.
// TODO: this does not handle multi-line responses properly
func (a *AGI) Command(cmd ...string) (resp *Response) {
	resp = &Response{}

	a.mu.Lock()
	defer a.mu.Unlock()

	cmdString := strings.Join(cmd, " ") + "\n"
	_, err := a.w.Write([]byte(cmdString))
	if err != nil {
		resp.Error = errors.Wrap(err, "failed to send command")
		return
	}

	s := bufio.NewScanner(a.r)
	for s.Scan() {
		raw := s.Text()
		if raw == "" {
			break
		}

		// Parse and store the result code
		pieces := responseRegex.FindStringSubmatch(raw)
		if pieces == nil {
			resp.Error = fmt.Errorf("failed to parse result: %s", raw)
			return
		}

		// Status code is the first substring
		resp.Status, err = strconv.Atoi(pieces[1])
		if err != nil {
			resp.Error = errors.Wrap(err, "failed to get status code")
			return
		}

		// Result code is the second substring
		resp.ResultString = pieces[2]
		resp.Result, err = strconv.Atoi(pieces[2])
		if err != nil {
			resp.Error = errors.Wrap(err, "failed to parse status code as an integer")
			return
		}

		// Value is the third (and optional) substring
		wrappedVal := strings.TrimSpace(pieces[3])
		resp.Value = strings.TrimSuffix(strings.TrimPrefix(wrappedVal, "("), ")")

		// FIXME: handle multiple line return values
		break // nolint
	}

	// If the Status code is not 200, return an error
	if resp.Status != 200 {
		resp.Error = fmt.Errorf("Non-200 status code")
	}
	return
}

// Answer answers the channel
func (a *AGI) Answer() error {
	return a.Command("ANSWER").Err()
}

// Status returns the channel status
func (a *AGI) Status() (State, error) {
	r, err := a.Command("CHANNEL STATUS").Val()
	if err != nil {
		return StateDown, err
	}
	state, err := strconv.Atoi(r)
	if err != nil {
		return StateDown, fmt.Errorf("Failed to parse state %s", r)
	}
	return State(state), nil
}

// Exec runs a dialplan application
func (a *AGI) Exec(cmd ...string) (string, error) {
	cmd = append([]string{"EXEC"}, cmd...)
	return a.Command(cmd...).Val()
}

// Get gets the value of the given channel variable
func (a *AGI) Get(key string) (string, error) {
	return a.Command("GET VARIABLE", key).Val()
}

// GetData plays a file and receives DTMF, returning the received digits
func (a *AGI) GetData(name string, timeout int, maxdigits int) (digit string, err error) {
	return a.Command("GET DATA", name, strconv.Itoa(timeout), strconv.Itoa(maxdigits)).Val()
}

// Hangup terminates the call
func (a *AGI) Hangup() error {
	return a.Command("HANGUP").Err()
}

// RecordOptions describes the options available when recording
type RecordOptions struct {
	// Format is the format of the audio file to record; defaults to "wav".
	Format string

	// EscapeDigits is the set of digits on receipt of which will terminate the recording. Default is "#".  This may not be blank.
	EscapeDigits string

	// Timeout is the maximum time to allow for the recording.  Defaults to 5 minutes.
	Timeout time.Duration

	// Silence is the maximum amount of silence to allow before ending the recording.  The finest resolution is to the second.   0=disabled, which is the default.
	Silence time.Duration

	// Beep controls whether a beep is played before starting the recording.  Defaults to false.
	Beep bool

	// Offset is the number of samples in the recording to advance before storing to the file.  This is means of clipping the beginning of a recording.  Defaults to 0.
	Offset int
}

// Record records audio to a file
func (a *AGI) Record(name string, opts *RecordOptions) error {
	if opts == nil {
		opts = &RecordOptions{}
	}
	if opts.Format == "" {
		opts.Format = "wav"
	}
	if opts.EscapeDigits == "" {
		opts.EscapeDigits = "#"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}

	cmd := strings.Join([]string{
		"RECORD FILE ",
		name,
		opts.Format,
		opts.EscapeDigits,
		toMSec(opts.Timeout),
	}, " ")

	if opts.Offset > 0 {
		cmd += " " + strconv.Itoa(opts.Offset)
	}

	if opts.Beep {
		cmd += " BEEP"
	}

	if opts.Silence > 0 {
		cmd += " s=" + toSec(opts.Silence)
	}

	return a.Command(cmd).Err()
}

// SayAlpha plays a character string, annunciating each character.
func (a *AGI) SayAlpha(label string, escapeDigits string) (digit string, err error) {
	return a.Command("SAY ALPHA", label, escapeDigits).Val()
}

// SayDigits plays a digit string, annunciating each digit.
func (a *AGI) SayDigits(number string, escapeDigits string) (digit string, err error) {
	return a.Command("SAY DIGITS", number, escapeDigits).Val()
}

// SayDate plays a date
func (a *AGI) SayDate(when time.Time, escapeDigits string) (digit string, err error) {
	return a.Command("SAY DATE", toEpoch(when), escapeDigits).Val()
}

// SayDateTime plays a date using the given format.  See `voicemail.conf` for the format syntax; defaults to `ABdY 'digits/at' IMp`.
func (a *AGI) SayDateTime(when time.Time, escapeDigits string, format string) (digit string, err error) {
	// Extract the timezone from the time
	zone, _ := when.Zone()

	// Use the Asterisk default format if we are not given one
	if format == "" {
		format = "ABdY 'digits/at' IMp"
	}

	return a.Command("SAY DATETIME", toEpoch(when), escapeDigits, format, zone).Val()
}

// SayNumber plays the given number.
func (a *AGI) SayNumber(number string, escapeDigits string) (digit string, err error) {
	return a.Command("SAY NUMBER", number, escapeDigits).Val()
}

// SayPhonetic plays the given phrase phonetically
func (a *AGI) SayPhonetic(phrase string, escapeDigits string) (digit string, err error) {
	return a.Command("SAY PHOENTIC", phrase, escapeDigits).Val()
}

// SayTime plays the time part of the given timestamp
func (a *AGI) SayTime(when time.Time, escapeDigits string) (digit string, err error) {
	return a.Command("SAY TIME", toEpoch(when), escapeDigits).Val()
}

// Set sets the given channel variable to
// the provided value.
func (a *AGI) Set(key, val string) error {
	return a.Command("SET VARIABLE", key, val).Err()
}

// StreamFile plays the given file to the channel
func (a *AGI) StreamFile(name string, escapeDigits string, offset int) (digit string, err error) {
	return a.Command("STREAM FILE", name, escapeDigits, strconv.Itoa(offset)).Val()
}

// Verbose logs the given message to the verbose message system
func (a *AGI) Verbose(msg string, level int) error {
	return a.Command("VERBOSE", msg, strconv.Itoa(level)).Err()
}

// WaitForDigit waits for a DTMF digit and returns what is received
func (a *AGI) WaitForDigit(timeout time.Duration) (digit string, err error) {
	return a.Command("WAIT FOR DIGIT", toMSec(timeout)).Val()
}
