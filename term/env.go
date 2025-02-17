package term

type color uint8

const (
	red color = iota
	black
)

// Env is a mapping from variables to terms.
type Env struct {
	// basically, this is Red-Black tree from Purely Functional Data Structures by Okazaki.
	color       color
	left, right *Env
	binding
}

type binding struct {
	variable Variable
	value    Interface
	// attributes?
}

// NewEnv creates an empty environment.
func NewEnv() *Env {
	return nil
}

// Lookup returns a term that the given variable is bound to.
func (e *Env) Lookup(k Variable) (Interface, bool) {
	node := e
	for {
		if node == nil {
			return nil, false
		}
		switch {
		case k < node.variable:
			node = node.left
		case k > node.variable:
			node = node.right
		default:
			return node.value, true
		}
	}
}

// Bind adds a new entry to the environment.
func (e *Env) Bind(k Variable, v Interface) *Env {
	ret := *e.insert(k, v)
	ret.color = black
	return &ret
}

func (e *Env) insert(k Variable, v Interface) *Env {
	if e == nil {
		return &Env{color: red, binding: binding{variable: k, value: v}}
	}
	switch {
	case k < e.variable:
		ret := *e
		ret.left = e.left.insert(k, v)
		ret.balance()
		return &ret
	case k > e.variable:
		ret := *e
		ret.right = e.right.insert(k, v)
		ret.balance()
		return &ret
	default:
		return e
	}
}

func (e *Env) balance() {
	var (
		a, b, c, d *Env
		x, y, z    binding
	)
	switch {
	case e.left != nil && e.left.color == red:
		switch {
		case e.left.left != nil && e.left.left.color == red:
			a = e.left.left.left
			b = e.left.left.right
			c = e.left.right
			d = e.right
			x = e.left.left.binding
			y = e.left.binding
			z = e.binding
		case e.left.right != nil && e.left.right.color == red:
			a = e.left.left
			b = e.left.right.left
			c = e.left.right.right
			d = e.right
			x = e.left.binding
			y = e.left.right.binding
			z = e.binding
		default:
			return
		}
	case e.right != nil && e.right.color == red:
		switch {
		case e.right.left != nil && e.right.left.color == red:
			a = e.left
			b = e.right.left.left
			c = e.right.left.right
			d = e.right.right
			x = e.binding
			y = e.right.left.binding
			z = e.right.binding
		case e.right.right != nil && e.right.right.color == red:
			a = e.left
			b = e.right.left
			c = e.right.right.left
			d = e.right.right.right
			x = e.binding
			y = e.right.binding
			z = e.right.right.binding
		default:
			return
		}
	default:
		return
	}
	*e = Env{
		color:   red,
		left:    &Env{color: black, left: a, right: b, binding: x},
		right:   &Env{color: black, left: c, right: d, binding: z},
		binding: y,
	}
}

// Resolve follows the variable chain and returns the first non-variable term or the last free variable.
func (e *Env) Resolve(t Interface) Interface {
	var stop []Variable
	for t != nil {
		switch v := t.(type) {
		case Variable:
			for _, s := range stop {
				if v == s {
					return v
				}
			}
			ref, ok := e.Lookup(v)
			if !ok {
				return v
			}
			stop = append(stop, v)
			t = ref
		default:
			return v
		}
	}
	return nil
}

// Simplify trys to remove as many variables as possible from term t.
func (e *Env) Simplify(t Interface) Interface {
	switch t := e.Resolve(t).(type) {
	case *Compound:
		c := Compound{
			Functor: t.Functor,
			Args:    make([]Interface, len(t.Args)),
		}
		for i := 0; i < len(c.Args); i++ {
			c.Args[i] = e.Simplify(t.Args[i])
		}
		return &c
	default:
		return t
	}
}

type Variables []Variable

func (vs Variables) Terms() []Interface {
	res := make([]Interface, len(vs))
	for i, v := range vs {
		res[i] = v
	}
	return res
}

func (vs Variables) Except(ws Variables) Variables {
	ret := make(Variables, 0, len(vs))
vs:
	for _, v := range vs {
		for _, w := range ws {
			if v == w {
				continue vs
			}
		}
		ret = append(ret, v)
	}
	return ret
}

// FreeVariables extracts variables in the given terms.
func (e *Env) FreeVariables(ts ...Interface) Variables {
	var fvs Variables
	for _, t := range ts {
		fvs = e.appendFreeVariables(fvs, t)
	}
	return fvs
}

func (e *Env) appendFreeVariables(fvs Variables, t Interface) Variables {
	switch t := e.Resolve(t).(type) {
	case Variable:
		for _, v := range fvs {
			if v == t {
				return fvs
			}
		}
		return append(fvs, t)
	case *Compound:
		for _, arg := range t.Args {
			fvs = e.appendFreeVariables(fvs, arg)
		}
	}
	return fvs
}
