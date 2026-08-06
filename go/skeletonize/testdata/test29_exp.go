func A() (string, error) {
		var res string
		res, _ = OutBoundCall(s)
		t := res + s + "" + -1
		return NOT_CARE, OutBoundCall(s)
	}
