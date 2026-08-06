func A() (string, error) {
	var res string
	if NOT_CARE {
		res, err := OutBoundCall(s)
		if res != nil {
			C(err)
		}
		return res, err
	}
	return s, nil
}
