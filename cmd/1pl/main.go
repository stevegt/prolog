package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/ichiban/prolog/syntax"

	"github.com/ichiban/prolog/engine"

	"github.com/ichiban/prolog/nondet"
	"github.com/ichiban/prolog/term"

	"github.com/ichiban/prolog"

	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"
)

// Version is a version of this build.
var Version = "1pl/0.1"

func main() {
	var verbose, debug bool
	var cd string
	pflag.BoolVarP(&verbose, "verbose", "v", false, `verbose`)
	pflag.BoolVarP(&debug, "debug", "d", false, `debug`)
	pflag.StringVarP(&cd, "cd", "c", "", `cd`)
	pflag.Parse()

	oldState, err := terminal.MakeRaw(0)
	if err != nil {
		log.Panicf("failed to enter raw mode: %v", err)
	}
	restore := func() {
		_ = terminal.Restore(0, oldState)
	}
	defer restore()

	t := terminal.NewTerminal(os.Stdin, "?- ")
	defer fmt.Printf("\r\n")

	log.SetOutput(t)

	i := prolog.New(bufio.NewReader(os.Stdin), t)
	i.Register1("halt", func(t term.Interface, k func(*term.Env) *nondet.Promise, env *term.Env) *nondet.Promise {
		restore()
		return engine.Halt(t, k, env)
	})
	i.Register1("cd", func(dir term.Interface, k func(*term.Env) *nondet.Promise, env *term.Env) *nondet.Promise {
		switch dir := env.Resolve(dir).(type) {
		case term.Atom:
			if err := os.Chdir(string(dir)); err != nil {
				return nondet.Error(err)
			}
			return k(env)
		default:
			return nondet.Error(errors.New("not a dir"))
		}
	})
	if verbose {
		i.OnCall = func(pi engine.ProcedureIndicator, args []term.Interface, env *term.Env) {
			goal, err := pi.Apply(args)
			if err != nil {
				log.Print(err)
			}
			log.Printf("CALL %s", i.DescribeTerm(goal, env))
		}
		i.OnExit = func(pi engine.ProcedureIndicator, args []term.Interface, env *term.Env) {
			goal, err := pi.Apply(args)
			if err != nil {
				log.Print(err)
			}
			log.Printf("EXIT %s", i.DescribeTerm(goal, env))
		}
		i.OnFail = func(pi engine.ProcedureIndicator, args []term.Interface, env *term.Env) {
			goal, err := pi.Apply(args)
			if err != nil {
				log.Print(err)
			}
			log.Printf("FAIL %s", i.DescribeTerm(goal, env))
		}
		i.OnRedo = func(pi engine.ProcedureIndicator, args []term.Interface, env *term.Env) {
			goal, err := pi.Apply(args)
			if err != nil {
				log.Print(err)
			}
			log.Printf("REDO %s", i.DescribeTerm(goal, env))
		}
	}
	i.OnUnknown = func(pi engine.ProcedureIndicator, args []term.Interface, env *term.Env) {
		log.Printf("UNKNOWN %s", pi)
	}
	i.Register1("version", func(t term.Interface, k func(*term.Env) *nondet.Promise, env *term.Env) *nondet.Promise {
		env, ok := t.Unify(term.Atom(Version), false, env)
		if !ok {
			return nondet.Bool(false)
		}
		return k(env)
	})

	for _, a := range pflag.Args() {
		b, err := ioutil.ReadFile(a)
		if err != nil {
			log.Panicf("failed to read %s: %v", a, err)
		}

		if err := i.Exec(string(b)); err != nil {
			log.Panicf("failed to execute %s: %v", a, err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var buf strings.Builder
	keys := bufio.NewReader(os.Stdin)
	for {
		if err := handleLine(ctx, &buf, i, t, keys); err != nil {
			log.Panic(err)
		}
	}
}

func handleLine(ctx context.Context, buf *strings.Builder, i *prolog.Interpreter, t *terminal.Terminal, keys *bufio.Reader) error {
	if buf.Len() == 0 {
		t.SetPrompt("?- ")
	} else {
		t.SetPrompt("|  ")
	}

	line, err := t.ReadLine()
	if err != nil {
		if err == io.EOF {
			return err
		}
		log.Printf("failed to read line: %v", err)
		buf.Reset()
		return nil
	}
	if _, err := buf.WriteString(line); err != nil {
		log.Printf("failed to buffer: %v", err)
		buf.Reset()
		return nil
	}

	c := 0
	sols, err := i.QueryContext(ctx, buf.String())
	switch err {
	case nil:
		break
	case syntax.ErrInsufficient:
		if _, err := buf.WriteRune('\n'); err != nil {
			log.Printf("failed to buffer: %v", err)
			buf.Reset()
		}

		// Returns without resetting buf.
		return nil
	default:
		log.Printf("failed to query: %v", err)
		buf.Reset()
		return nil
	}

	for sols.Next() {
		c++

		m := map[string]term.Interface{}
		if err := sols.Scan(m); err != nil {
			log.Printf("failed to scan: %v", err)
			break
		}

		vars := sols.Vars()
		ls := make([]string, 0, len(vars))
		for _, n := range vars {
			v := m[n]
			if _, ok := v.(term.Variable); ok {
				continue
			}
			ls = append(ls, fmt.Sprintf("%s = %s", n, v))
		}
		if len(ls) == 0 {
			if _, err := fmt.Fprintf(t, "%t.\n", true); err != nil {
				return err
			}
			break
		}

		if _, err := fmt.Fprintf(t, "%s ", strings.Join(ls, ",\n")); err != nil {
			return err
		}

		r, _, err := keys.ReadRune()
		if err != nil {
			log.Printf("failed to read rune: %v", err)
			break
		}
		if r != ';' {
			r = '.'
		}

		if _, err := fmt.Fprintf(t, "%s\n", string(r)); err != nil {
			return err
		}

		if r == '.' {
			break
		}
	}
	if err := sols.Close(); err != nil {
		return err
	}

	if err := sols.Err(); err != nil {
		log.Printf("failed: %v", err)
		buf.Reset()
		return nil
	}

	if c == 0 {
		if _, err := fmt.Fprintf(t, "%t.\n", false); err != nil {
			return err
		}
	}

	buf.Reset()
	return nil
}
