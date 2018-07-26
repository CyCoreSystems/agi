package agi

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// RecognitionResult describes the result of an MRCP speech recognition action
type RecognitionResult struct {

	// Status indicates the value of RECOG_STATUS, which is one of "OK", "ERROR", or "INTERRUPTED", which indicates whether the recognition process completed.
	//
	//  "OK" - the recognition executed properly
	//
	//  "ERROR" - the recognition failed to execute
	//
	//  "INTERRUPTED" - the call ended before the recognition could complete its execution
	//
	Status string

	// Cause indicates the value of RECOG_COMPLETION_CAUSE, which indicates whether speech was recognized.
	//
	// Possible values are:
	//
	// 0 - Success; speech was recognized
	//
	// 1 - No Match; speech was detected but it did not match anything in the grammar
	//
	// 2 - No Input; no speech was detected
	//
	Cause int

	// Result is the value of RECOG_RESULT, which contains the NLSML result (unparsed string) received from the MRCP server.
	Result string
}

// RecognitionInterpretation describes a specific interpretation of speech input
type RecognitionInterpretation struct {

	// Confidence indicates how sure the MRCP server's engine was that the result was properly recognized.  It is a value from 0-100, with the highest value indicating the most confidence.
	Confidence int

	// Input is the textual representation of the recognized speech
	Input string

	// Grammar indicates the grammar or recognition rule which was matched
	Grammar string
}

// getRecognitionResult retrieves the set of channel variables which comprises the recognition result of a speech recognition MRCP session.  The "combo" parameter indicates whether the process was the SynthAndRecog combo application, which stored the STATUS differently from the singular MRCPSynth.
func (a *AGI) getRecognitionResult(combo bool) (res *RecognitionResult, err error) {
	var cause string
	res = new(RecognitionResult)

	statusVar := "RECOGSTATUS"
	if combo {
		statusVar = "RECOG_STATUS"
	}

	if res.Status, err = a.Get(statusVar); err != nil {
		return res, errors.Wrap(err, "failed to retrieve status")
	}
	if cause, err = a.Get("RECOG_COMPLETION_CAUSE"); err != nil {
		return res, errors.Wrap(err, "failed to retrieve cause")
	}
	if res.Cause, err = strconv.Atoi(cause); err != nil {
		return res, errors.Wrapf(err, "failed to parse cause (%s) as an integer", cause)
	}
	if res.Result, err = a.Get("RECOG_RESULT"); err != nil {
		return res, errors.Wrap(err, "failed to retrieve result")
	}

	return res, nil
}

// SynthResult describes the result of an MRCP Synthesis operation
type SynthResult struct {

	// Status indicates whether the operation completed.
	//
	// Valid values are:
	//
	//   - "OK" : the synthesis operation succeeded
	//
	//   - "ERROR" : the synthesis operation failed
	//
	//   - "INTERRUPTED" : the channel disappeared during the synthesis operation
	//
	Status string

	// Cause is a numeric code indicating the reason for the status
	//
	// Known values are:
	//
	//   - 0 : Normal
	//
	//   - 1 : Barge-In occurred
	//
	//   - 2 : Parse failure
	//
	Cause int
}

// MRCPSynth synthesizes speech for a prompt via MRCP. (requires UniMRCP app and resource to be compiled and loaded in Asterisk).
func (a *AGI) MRCPSynth(prompt string, opts string) (res *SynthResult, err error) {
	var cause string
	res = new(SynthResult)

	ret, err := a.Exec([]string{"MRCPSynth", prompt, opts}...)
	if err != nil {
		return
	}
	if ret == "-2" {
		return res, errors.New("MRCP applications not loaded")
	}

	if res.Status, err = a.Get("SYNTHSTATUS"); err != nil {
		return res, errors.Wrap(err, "failed to retrieve status")
	}
	if cause, err = a.Get("SYNTH_COMPLETION_CAUSE"); err != nil {
		return res, errors.Wrap(err, "failed to retrieve cause")
	}
	if res.Cause, err = strconv.Atoi(cause); err != nil {
		return res, errors.Wrapf(err, "failed to parse cause (%s) as an integer", cause)
	}

	return
}

// MRCPRecog listens for speech and optionally plays a prompt. (requires UniMRCP app and resource to be compiled and loaded in Asterisk).
func (a *AGI) MRCPRecog(grammar string, opts string) (*RecognitionResult, error) {

	ret, err := a.Exec([]string{"MRCPRecog", grammar, opts}...)
	if err != nil {
		return nil, err
	}
	if ret == "-2" {
		return nil, errors.New("MRCP applications not loaded")
	}

	return a.getRecognitionResult(false)
}

// SynthAndRecog plays a synthesized prompt and waits for speech to be recognized (requires UniMRCP app and resource to be compiled and loaded in Asterisk).
func (a *AGI) SynthAndRecog(prompt string, grammar string, opts string) (*RecognitionResult, error) {

	execOpts := []string{
		fmt.Sprintf(`"%s"`, prompt),
		grammar,
		opts,
	}
	ret, err := a.Exec([]string{"SynthAndRecog", strings.Join(execOpts, ",")}...)
	if err != nil {
		return nil, err
	}
	if ret == "-2" {
		return nil, errors.New("MRCP applications not loaded")
	}

	return a.getRecognitionResult(true)
}

// RecognitionInterpretation returns the speech interpretation from the last MRCP speech recognition process.  The index is based on the set of results ordered by decreasing confidence.  Thus index 0 is the best match.
func (a *AGI) RecognitionInterpretation(index int) (ret *RecognitionInterpretation, err error) {
	ret = new(RecognitionInterpretation)

	if ret.Input, err = a.RecognitionInput(index); err != nil {
		return
	}
	if ret.Confidence, err = a.RecognitionConfidence(index); err != nil {
		return
	}
	if ret.Grammar, err = a.RecognitionGrammar(index); err != nil {
		return
	}
	return
}

// RecognitionInput returns the detected input from the last MRCP speech
// recognition process.  The index is based on the set of results ordered by
// decreasing confidence.  Thus index 0 is the best match.
func (a *AGI) RecognitionInput(index int) (string, error) {
	return a.Get(fmt.Sprintf("RECOG_INPUT(%d)", index))
}

// RecognitionConfidence returns the confidence level (0-100 with 100 being best) from the last MRCP speech recognition process.  The index is based on the set of results ordered by decreasing confidence.  Thus index 0 is the best match.
func (a *AGI) RecognitionConfidence(index int) (int, error) {
	out, err := a.Get(fmt.Sprintf("RECOG_CONFIDENCE(%d)", index))
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(out)
}

// RecognitionGrammar returns the grammar which was matched from the last MRCP speech recognition process.  The index is based on the set of result ordered by decreasing confidence.  Thus index 0 is the best match.
func (a *AGI) RecognitionGrammar(index int) (string, error) {
	return a.Get(fmt.Sprintf("RECOG_GRAMMAR(%d)", index))
}
