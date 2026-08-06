func A() (string, error) {
	res, err := OutBoundCall(s)

	if NOT_CARE {
		res, err := B(res)
		if res != nil {
			C(err)
		}
		return res, err
	}

	for i := 0; i < 10; i++ {
		res := res + 1
		B(res)
	}

	return NOT_CARE, nil
}
