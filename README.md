[![GoDoc](https://godoc.org/github.com/michaelmaltese/golang-distributed-filesystem?status.png)](https://godoc.org/github.com/michaelmaltese/golang-distributed-filesystem) [![Build Status](https://travis-ci.org/michaelmaltese/golang-distributed-filesystem.svg?branch=master)](https://travis-ci.org/michaelmaltese/golang-distributed-filesystem)

Writing a HDFS clone in [Go](http://golang.org) to learn more about Go and the nitty-gritty of distributed systems. If you're hiring for data science / pipeline-engineering jobs, or want to talk, I'd love if you got in touch with me [@michaelmaltese](https://twitter.com/michaelmaltese) or [mjm72010@mymail.pomona.edu](mailto:mjm72010@mymail.pomona.edu).

## Features/TODO

- [x] MetaDataNode/DataNode handle uploads
- [x] MetaDataNode/DataNode handle downloads
- [x] DataNode dynamically registers with MetaDataNode
- [x] DataNode tells MetaDataNode its blocks on startup
- [x] MetaDataNode persists file->blocklist map
- [x] DataNode pipelines uploads to other DataNodes
- [x] MetaDataNode can restart and DataNode will re-register (heartbeats)
- [x] Tell DataNodes to re-register if MetaDataNode doesn't recognize them
- [x] Drop DataNodes when they go down (heartbeats)
- [x] DataNode sends size of data directory (heartbeat)
- [x] MetaDataNode obeys replication factor
- [x] MetaDataNode balances based on current reported space
- [x] MetaDataNode balances based on expected new blocks
- [ ] Balance new DataNodes (need to keep a plan of what we're doing / what blocks to expect where?)
- [ ] MetaDataNode balances blocks as it runs
- [ ] If a client tries to upload a block and every DataNode in its list is down, it needs to get more from the MetaDataNode.
- [ ] Support multiple MetaDataNodes with a DHT (and consensus algorithm?)