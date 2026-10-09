// Command mldsa_vectors reads the pinned ML-DSA vectors (Wycheproof under toolchain/tests/wycheproof/mldsa, and the
// NIST ACVP files under toolchain/tests/data/mldsa/acvp) and prints one case per line: the source, the case in the
// fixture's input form, and the answer the vector gives, separated by tabs. The check tools/ci/mldsa_check.tin feeds
// the cases to tools/ci/fixtures/mldsa.tin and compares. This program only copies the vectors into that form (Tin's
// JSON reader needs far more memory than the files take); it computes nothing. It needs only Go 1.26, so CI can run it.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	wycheproof = "toolchain/tests/wycheproof/mldsa"
	acvp       = "toolchain/tests/data/mldsa/acvp"
)

// wpVerify and wpSign are the Wycheproof files' fields that the cases use.
type wpVerify struct {
	TestGroups []struct {
		PublicKey string `json:"publicKey"`
		Tests     []struct {
			Msg    string `json:"msg"`
			Ctx    string `json:"ctx"`
			Sig    string `json:"sig"`
			Result string `json:"result"`
		} `json:"tests"`
	} `json:"testGroups"`
}

type wpSign struct {
	TestGroups []struct {
		PrivateSeed string `json:"privateSeed"`
		Tests       []struct {
			Msg    string `json:"msg"`
			Ctx    string `json:"ctx"`
			Mu     string `json:"mu"`
			Rnd    string `json:"rnd"`
			Sig    string `json:"sig"`
			Result string `json:"result"`
		} `json:"tests"`
	} `json:"testGroups"`
}

// acvpPrompt holds the inputs of a mode's test groups; acvpExpected the outputs, in the same order.
type acvpPrompt struct {
	TestGroups []struct {
		ParameterSet       string `json:"parameterSet"`
		SignatureInterface string `json:"signatureInterface"`
		Tests              []struct {
			TcID    int64  `json:"tcId"`
			Seed    string `json:"seed"`
			Sk      string `json:"sk"`
			Pk      string `json:"pk"`
			Message string `json:"message"`
			Context string `json:"context"`
			Mu      string `json:"mu"`
			Rnd     string `json:"rnd"`
			Sig     string `json:"signature"`
		} `json:"tests"`
	} `json:"testGroups"`
}

type acvpExpected struct {
	TestGroups []struct {
		Tests []struct {
			TcID       int64  `json:"tcId"`
			Pk         string `json:"pk"`
			Sk         string `json:"sk"`
			Signature  string `json:"signature"`
			TestPassed bool   `json:"testPassed"`
		} `json:"tests"`
	} `json:"testGroups"`
}

// field is a hex field of a case line: lowercase, and "-" when it is empty.
func field(s string) string {
	if s == "" {
		return "-"
	}
	return strings.ToLower(s)
}

// setOf is the parameter set's number ("ML-DSA-44" gives "44").
func setOf(name string) string {
	return strings.TrimPrefix(name, "ML-DSA-")
}

// hexWant is the expected output of a case: its bytes in lowercase hex.
func hexWant(s string) string {
	return "hex:" + strings.ToLower(s)
}

// read decodes a JSON file of the pinned vectors into v.
func read(path string, v any) {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := json.NewDecoder(bufio.NewReaderSize(f, 1<<20)).Decode(v); err != nil {
		panic(fmt.Sprintf("%s: %v", path, err))
	}
}

// emit prints one case: its source, the case line and the answer it must have.
func emit(source, line, want string) {
	fmt.Printf("%s\t%s\t%s\n", source, line, want)
}

func main() {
	for _, n := range []string{"44", "65", "87"} {
		var v wpVerify
		read(filepath.Join(wycheproof, "mldsa_"+n+"_verify_test.json"), &v)
		for _, g := range v.TestGroups {
			pk := field(g.PublicKey)
			for _, t := range g.Tests {
				line := "vfy " + n + " " + pk + " " + field(t.Msg) + " " + field(t.Ctx) + " " + field(t.Sig)
				want := "fault"
				if t.Result == "valid" {
					want = "ok"
				}
				emit("wycheproof verify "+n, line, want)
			}
		}
		var s wpSign
		read(filepath.Join(wycheproof, "mldsa_"+n+"_sign_seed_test.json"), &s)
		for _, g := range s.TestGroups {
			seed := field(g.PrivateSeed)
			for _, t := range g.Tests {
				line := "sign " + n + " " + seed + " " + field(t.Msg) + " " + field(t.Ctx) + " " + field(t.Rnd)
				if t.Mu != "" {
					line = "mu " + n + " " + seed + " " + field(t.Mu) + " " + field(t.Rnd)
				}
				want := "fault"
				if t.Result == "valid" {
					want = hexWant(t.Sig)
				}
				emit("wycheproof sign "+n, line, want)
			}
		}
	}
	keyGenPrompt, keyGenExpected := acvpFiles("keyGen")
	for gi, g := range keyGenPrompt.TestGroups {
		n := setOf(g.ParameterSet)
		for ti, t := range g.Tests {
			e := keyGenExpected.TestGroups[gi].Tests[ti]
			if e.TcID != t.TcID {
				panic("ACVP keyGen: tcId order differs")
			}
			seed := field(t.Seed)
			emit("acvp keyGen "+n, "pk "+n+" "+seed, hexWant(e.Pk))
			emit("acvp keyGen "+n, "esk "+n+" "+seed, hexWant(e.Sk))
		}
	}
	sigGenPrompt, sigGenExpected := acvpFiles("sigGen")
	for gi, g := range sigGenPrompt.TestGroups {
		n := setOf(g.ParameterSet)
		for ti, t := range g.Tests {
			e := sigGenExpected.TestGroups[gi].Tests[ti]
			if e.TcID != t.TcID {
				panic("ACVP sigGen: tcId order differs")
			}
			sk := field(t.Sk)
			line := "esign " + n + " " + sk + " " + field(t.Message) + " " + field(t.Context) + " " + field(t.Rnd)
			if t.Mu != "" {
				line = "emu " + n + " " + sk + " " + field(t.Mu) + " " + field(t.Rnd)
			} else if g.SignatureInterface == "internal" {
				line = "eint " + n + " " + sk + " " + field(t.Message) + " " + field(t.Rnd)
			}
			emit("acvp sigGen "+n, line, hexWant(e.Signature))
		}
	}
	sigVerPrompt, sigVerExpected := acvpFiles("sigVer")
	for gi, g := range sigVerPrompt.TestGroups {
		n := setOf(g.ParameterSet)
		for ti, t := range g.Tests {
			e := sigVerExpected.TestGroups[gi].Tests[ti]
			if e.TcID != t.TcID {
				panic("ACVP sigVer: tcId order differs")
			}
			pk := field(t.Pk)
			sig := field(t.Sig)
			line := "vfy " + n + " " + pk + " " + field(t.Message) + " " + field(t.Context) + " " + sig
			if t.Mu != "" {
				line = "vmu " + n + " " + pk + " " + field(t.Mu) + " " + sig
			} else if g.SignatureInterface == "internal" {
				line = "vint " + n + " " + pk + " " + field(t.Message) + " " + sig
			}
			want := "fault"
			if e.TestPassed {
				want = "ok"
			}
			emit("acvp sigVer "+n, line, want)
		}
	}
}

// acvpFiles reads the prompt and the expected results of one ACVP mode.
func acvpFiles(mode string) (acvpPrompt, acvpExpected) {
	var p acvpPrompt
	var e acvpExpected
	read(filepath.Join(acvp, mode+"_prompt.json"), &p)
	read(filepath.Join(acvp, mode+"_expected.json"), &e)
	if len(p.TestGroups) != len(e.TestGroups) {
		panic("ACVP " + mode + ": the prompt and the expected results differ in groups")
	}
	return p, e
}
