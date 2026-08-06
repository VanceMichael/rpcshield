func A() string {
	res, err := OutBoundCall(s)
	ch <- err
	value := <-ch
	return NOT_CARE
}

