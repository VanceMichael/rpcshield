struct Message {
    1: required string content
}

// SimpleService is a second, Thrift-based inbound transport for the demo
// handler (yarpc's thrift encoding), analogous to cff-demo's TChannel handler.
service SimpleService {
    Message create(1: Message msg)
}
