# Causal Tree RDT

Implementation of a causal tree replicated data type (RDT) in Go.

## Requirements

We use Go 1.18 for its fuzzing capabilities.

## Repository structure

- `crdt/`: replicated data type implementation
- `diff/`: string diff implementation
- `debug/`: web viewer of CRDT structure
- `cmd/demo/`: demo server
- `bench/`: benchmark results and analysis

## Run demo

The demo server allows you to play around with concurrent text CRDTs. To run the demo server,
execute the following from the repo root:

    $ go run cmd/demo/demo.go --debug_dir debug/ --static_dir cmd/demo/static/ --debug
    Serving in :8009

The web interface at http://localhost:8009 allows you to edit multiple structures in the same page, by forking lists
into separate sites, and sync'ing them to test the automatic merge capabilities. One can also
change which sites are used to sync from.

![Web interface of demo server](/docs/demo-server.png)

## Viewing data structure

If run in `--debug` mode, the demo server keeps a log of all operations in JSONL format. This log
can be visualized at http://localhost:8009/debug. Tests also write a log in the `testdata/` directory.
This webpage can be served independently of a demo server, for example, by running `python -m http.server` from the `debug/`
directory.

![Web interface of CRDT viewer](/docs/crdt-viewer.png)

## Run tests

To run all the test packages of the project, execute the following command from the repo root:
    $ go test ./...

