# RPCShield

RPCShield is a whole-program static analyzer for microservices that determines, for each
dependency a service makes on another, whether a failure of the downstream dependency also causes
the calling (upstream) service to fail: **fail-close**, where the downstream failure does cause the
caller to fail, or **fail-open**, where it does not. It builds a whole-program call graph using a
custom Rapid Type Analysis (RTA) implementation, then, for every outbound call, walks the call
chain back to each inbound entry point that can reach it, analyzing one caller/callee hop at a time
to determine whether a failure at that call site causes the entry point to fail. This applies to
any dependency, but is most useful where a dependency crosses criticality tiers (a more-critical
service calling a less-critical one), since that's where a downstream failure causing an upstream
failure is most consequential: it couples the critical service's own availability to a dependency
that wasn't built to the same reliability bar.

This classification supports several use cases, including:

- Identifying fail-close dependencies between services of different reliability tiers, where a
  more-critical service's availability ends up coupled to a less-critical dependency.
- Setting retry budgets appropriately: a fail-close dependency benefits from a retry budget, while a
  fail-open dependency doesn't need one, since its failure doesn't cause the caller to fail.

The current implementation (the `go/` incarnation) analyzes Go microservices.

The Go module, source, and full technical documentation live under [`go/`](go/README.md).

## Getting started

See [`go/README.md`](go/README.md) for how the analysis pipeline works, the repository layout, and
build/test instructions, and [`go/CONTRIBUTING.md`](go/CONTRIBUTING.md) for how to contribute.

## Citing this work

If you use RPCShield in your research, please cite:

> Milind Chabbi, Sonal Mahajan, Ivan Beschastnikh, René Just, Yuxin Wang, Yufan Xu, Elton Pinto,
> Seemanta Saha, Manu Sridharan, Abhishek Jha, Sandeep Koushik Sheshadri, and Mayank Bansal.
> **RPCShield: Defending Microservices Against Cascading Failures.** In *Proceedings of the 32nd
> ACM Symposium on Operating Systems Principles (SOSP '26)*, 2026.

### BibTeX

```bibtex
@inproceedings{chabbi2026rpcshield,
  title     = {{RPCShield}: Defending Microservices Against Cascading Failures},
  author    = {Chabbi, Milind and Mahajan, Sonal and Beschastnikh, Ivan and Just, Ren\'{e} and
               Wang, Yuxin and Xu, Yufan and Pinto, Elton and Saha, Seemanta and
               Sridharan, Manu and Jha, Abhishek and Sheshadri, Sandeep Koushik and
               Bansal, Mayank},
  booktitle = {Proceedings of the 32nd ACM Symposium on Operating Systems Principles},
  series    = {SOSP '26},
  year      = {2026},
  publisher = {ACM}
}
```

### ACM Reference Format

> Milind Chabbi, Sonal Mahajan, Ivan Beschastnikh, René Just, Yuxin Wang, Yufan Xu, Elton Pinto,
> Seemanta Saha, Manu Sridharan, Abhishek Jha, Sandeep Koushik Sheshadri, and Mayank Bansal. 2026.
> RPCShield: Defending Microservices Against Cascading Failures. In *Proceedings of the 32nd ACM
> Symposium on Operating Systems Principles (SOSP '26)*. ACM, New York, NY, USA.

## Code of Conduct

This project follows the [Uber Code of Conduct](CODE_OF_CONDUCT.md).

## License

Apache License 2.0 — see [`LICENSE.md`](LICENSE.md).
