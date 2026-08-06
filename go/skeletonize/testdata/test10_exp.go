func A() (string, error) {
	res, err := OutBoundCall(s)
	if NOT_CARE {
		res, err := B(res)
		if res != nil {
			C(err)
		}
		return res, err
	}

	return s, nil
}

