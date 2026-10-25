package ir

// WalkExpr recursively visits an expression tree and all nested sub-expressions.
func WalkExpr(e Expr, onExpr func(Expr)) {
	if onExpr != nil {
		onExpr(e)
	}
	if e.Left != nil {
		WalkExpr(*e.Left, onExpr)
	}
	if e.Right != nil {
		WalkExpr(*e.Right, onExpr)
	}
	for _, fi := range e.Fields {
		WalkExpr(fi.Value, onExpr)
	}
	for _, it := range e.Items {
		WalkExpr(it, onExpr)
	}
}

// WalkStepsAndExprs recursively traverses an Action Pipeline's steps (including nested
// if/else, match arms, and loops) and visits every step and expression.
func WalkStepsAndExprs(steps []StepDecl, onStep func(StepDecl), onExpr func(Expr)) {
	for _, s := range steps {
		if onStep != nil {
			onStep(s)
		}
		if s.Expr != nil {
			WalkExpr(*s.Expr, onExpr)
		}
		if s.WhereExpr != nil {
			WalkExpr(*s.WhereExpr, onExpr)
		} else if s.WhereValue != nil {
			WalkExpr(*s.WhereValue, onExpr)
		}
		if s.LimitExpr != nil {
			WalkExpr(*s.LimitExpr, onExpr)
		}
		if s.CursorExpr != nil {
			WalkExpr(*s.CursorExpr, onExpr)
		}
		for _, arg := range s.PolicyArgs {
			WalkExpr(arg, onExpr)
		}
		for _, arg := range s.SlotArgs {
			WalkExpr(arg, onExpr)
		}
		for _, fi := range s.Fields {
			WalkExpr(fi.Value, onExpr)
		}
		WalkStepsAndExprs(s.ThenSteps, onStep, onExpr)
		WalkStepsAndExprs(s.ElseSteps, onStep, onExpr)
		for _, arm := range s.MatchArms {
			WalkStepsAndExprs(arm.Steps, onStep, onExpr)
		}
	}
}
