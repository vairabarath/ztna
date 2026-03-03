# Proto Breaking Baseline

`baseline.binpb` is the reference image used by:

```bash
make proto-compat
```

When you intentionally make a breaking API change and want to accept it:

```bash
make proto-baseline
```

Then rerun `make proto-compat`.
