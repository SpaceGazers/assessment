package metadatanode

import (
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/nu7hatch/gouuid"

	. "github.com/michaelmaltese/golang-distributed-filesystem/comm"
)

type ReplicationIntents struct {
	startedAt map[BlockID]time.Time
	sentCommand map[BlockID]bool
	availableFrom map[BlockID][]NodeID
	forwardTo map[BlockID][]NodeID
}

func (self *ReplicationIntents) Add(block BlockID, from []NodeID, to []NodeID) {
	if self.InProgress(block) {
		log.Fatalln("Already replicating block '" + block + "'")
	}
	self.startedAt[block] = time.Now()
	self.availableFrom[block] = from
	self.forwardTo[block] = to
}

func (self *ReplicationIntents) Get(node NodeID) map[BlockID][]NodeID {
	actions := map[BlockID][]NodeID{}
	for block, nodes := range self.availableFrom {
		if self.sentCommand[block] {
			continue
		}
		for _, anode := range nodes {
			if anode == node {
				actions[block] = self.forwardTo[block]
				self.sentCommand[block] = true
			}
		}
	}
	return actions
}

func (self *ReplicationIntents) InProgress(block BlockID) bool {
	if time.Since(self.startedAt[block]) < 20 * time.Second {
		return true
	}
	delete(self.sentCommand, block)
	delete(self.startedAt, block)
	delete(self.availableFrom, block)
	delete(self.forwardTo, block)
	return false
}

type deletionIntent struct {
	startedAt time.Time
	sentCommand bool
	block BlockID
	node NodeID
}

type DeletionIntents struct {
	intents []*deletionIntent
}

func (self *DeletionIntents) Add(block BlockID, from []NodeID) {
	for _, node := range from {
		if self.InProgress(block) {
			log.Fatalln("Already deleting block '" + string(block) + "' from '" + string(node) + "'")
		}
		self.intents = append(self.intents, &deletionIntent{time.Now(), false, block, node})
	}
}

func (self *DeletionIntents) Count(node NodeID) int {
	var deletions []BlockID
	for _, intent := range self.intents {
		if intent.node == node {
			if time.Since(intent.startedAt) < 20 * time.Second {
				deletions = append(deletions, intent.block)
			}
		}
	}
	return len(deletions)
}

func (self *DeletionIntents) Get(node NodeID) []BlockID {
	var deletions []BlockID
	for _, intent := range self.intents {
		if intent.sentCommand {
			continue
		}
		if intent.node == node {
			deletions = append(deletions, intent.block)
			intent.sentCommand = true
			intent.startedAt = time.Now()
		}
	}
	return deletions
}

func (self *DeletionIntents) Done(node NodeID, block BlockID) {
	for i, intent := range self.intents {
		if intent.node == node && intent.block == block {
			self.intents = append(self.intents[:i], self.intents[i+1:]...)
		}
	}
}

func (self *DeletionIntents) InProgress(block BlockID) bool {
	for i, intent := range self.intents {
		if intent.block == block {
			if time.Since(intent.startedAt) < 20 * time.Second {
				return true
			}
			self.intents = append(self.intents[:i], self.intents[i+1:]...)
		}
	}
	return false
}

type MetaDataNodeState struct {
	mutex sync.Mutex
	store *DB
	dataNodes map[NodeID]string
	dataNodesLastSeen map[NodeID]time.Time
	dataNodesUtilization map[NodeID]int
	blocks map[BlockID]map[NodeID]bool
	dataNodesBlocks map[NodeID]map[BlockID]bool
	replicationIntents ReplicationIntents
	deletionIntents DeletionIntents
	ReplicationFactor int
}

func NewMetaDataNodeState() *MetaDataNodeState {
	var self MetaDataNodeState
	db, err := OpenDB("metadata.db")
	log.Println("Persistent storage at", "metadata.db")
	if err != nil {
		log.Fatalln("Metadata store error:", err)
	}
	self.store = db
	self.replicationIntents.startedAt = map[BlockID]time.Time{}
	self.replicationIntents.availableFrom = map[BlockID][]NodeID{}
	self.replicationIntents.forwardTo = map[BlockID][]NodeID{}
	self.replicationIntents.sentCommand = map[BlockID]bool{}

	self.dataNodesLastSeen = map[NodeID]time.Time{}
	self.dataNodes = map[NodeID]string{}
	self.dataNodesUtilization = map[NodeID]int{}
	self.blocks = map[BlockID]map[NodeID]bool{}
	self.dataNodesBlocks = map[NodeID]map[BlockID]bool{}
	return &self
}

func (self *MetaDataNodeState) GenerateBlobId() string {
	u4, err := uuid.NewV4()
	if err != nil {
		log.Fatal(err)
	}
	return u4.String()
}

func (self *MetaDataNodeState) GenerateBlockId() BlockID {
	u4, err := uuid.NewV4()
	if err != nil {
		log.Fatalln(err)
	}
	return BlockID(u4.String())
}

func (self *MetaDataNodeState) GetBlob(blobID string) []BlockID {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	names, err := self.store.Get(blobID)
	if err != nil {
		log.Fatalln(err)
	}

	// Is this really necessary?
	var blocks []BlockID
	for _, n := range names {
		blocks = append(blocks, BlockID(n))
	}

	return blocks
}

func (self *MetaDataNodeState) HasBlocks(nodeID NodeID, blocks []BlockID) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	for _, blockID := range blocks {
		if self.blocks[blockID] == nil {
			self.blocks[blockID] = map[NodeID]bool{}
		}
		if self.dataNodesBlocks[nodeID] == nil {
			self.dataNodesBlocks[nodeID] = map[BlockID]bool{}
		}
		self.blocks[blockID][nodeID] = true
		self.dataNodesBlocks[nodeID][blockID] = true
	}
}

func (self *MetaDataNodeState) DoesntHaveBlocks(nodeID NodeID, blockIDs []BlockID) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	for _, blockID := range blockIDs {
		self.deletionIntents.Done(nodeID, blockID)
		if self.blocks[blockID] != nil {
			delete(self.blocks[blockID], nodeID)
		}
		if self.dataNodesBlocks[nodeID] != nil {
			delete(self.dataNodesBlocks[nodeID], blockID)
		}
	}
}

func (self *MetaDataNodeState) GetBlock(blockID BlockID) []string {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	var addrs []string
	for nodeID, _ := range self.blocks[blockID] {
		addrs = append(addrs, self.dataNodes[nodeID])
	}

	return addrs
}

func (self *MetaDataNodeState) RegisterDataNode(addr string) NodeID {
	u4, err := uuid.NewV4()
	if err != nil {
		log.Fatal(err)
	}
	nodeID := NodeID(u4.String())

	self.mutex.Lock()
	defer self.mutex.Unlock()
	self.dataNodes[nodeID] = addr
	self.dataNodesUtilization[nodeID] = 0
	self.dataNodesLastSeen[nodeID] = time.Now()
	return nodeID
}

func (self *MetaDataNodeState) HeartbeatFrom(nodeID NodeID, utilization int) bool {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	self.dataNodesLastSeen[nodeID] = time.Now()
	self.dataNodesUtilization[nodeID] = utilization
	return len(self.dataNodes[nodeID]) > 0
}

type ByRandom []NodeID
func (s ByRandom) Len() int {
    return len(s)
}
func (s ByRandom) Swap(i, j int) {
    s[i], s[j] = s[j], s[i]
}
func (s ByRandom) Less(i, j int) bool {
    return rand.Intn(2) == 0 // 0 or 1
}

type ByUtilization []NodeID
func (s ByUtilization) Len() int {
    return len(s)
}
func (s ByUtilization) Swap(i, j int) {
    s[i], s[j] = s[j], s[i]
}
func (s ByUtilization) Less(i, j int) bool {
	iu := State.dataNodesUtilization[s[i]] - State.deletionIntents.Count(s[i])
	ju := State.dataNodesUtilization[s[j]] - State.deletionIntents.Count(s[j])
    return iu < ju
}


// This is not concurrency safe
func (self *MetaDataNodeState) LeastUsedNodes() []NodeID {
	var nodes []NodeID
	for nodeID, _ := range self.dataNodes {
		nodes = append(nodes, nodeID)
	}

	sort.Sort(ByRandom(nodes))
	sort.Stable(ByUtilization(nodes))

	return nodes
}
func (self *MetaDataNodeState) MostUsedNodes() []NodeID {
	var nodes []NodeID
	for nodeID, _ := range self.dataNodes {
		nodes = append(nodes, nodeID)
	}

	sort.Sort(ByRandom(nodes))
	sort.Stable(sort.Reverse(ByUtilization(nodes)))

	return nodes
}

func (self *MetaDataNodeState) GetDataNodes() []string {
	self.mutex.Lock()
	self.mutex.Unlock()
	nodes := self.LeastUsedNodes()
	
	var forwardTo []NodeID
	if len(nodes) < self.ReplicationFactor {
		forwardTo = nodes
	} else {
		forwardTo = nodes[0:self.ReplicationFactor]
	}

	var addrs []string
	for _, nodeID := range forwardTo {
		self.dataNodesUtilization[nodeID] += 1
		addrs = append(addrs, self.dataNodes[nodeID])
	}

	return addrs
}

func (self *MetaDataNodeState) CommitBlob(name string, blocks []BlockID) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	for _, b := range blocks {
		self.store.Append(name, string(b))
	}
}


func monitor() {
	for {
		log.Println("Monitor checking system..")
		// This sucks. Probably could do a separate lock for DataNodes and file stuff
		State.mutex.Lock()
		for id, lastSeen := range State.dataNodesLastSeen {
			if time.Since(lastSeen) > 20 * time.Second {
				log.Println("Forgetting absent node:", id)
				delete(State.dataNodesLastSeen, id)
				delete(State.dataNodes, id)
				delete(State.dataNodesUtilization, id)
				for block, _ := range State.dataNodesBlocks[id] {
					delete(State.blocks[block], id)
				}
				delete(State.dataNodesBlocks, id)
			}
		}

		for blockID, nodes := range State.blocks {
			switch {
			default:

			case State.replicationIntents.InProgress(blockID):
				continue

			case State.deletionIntents.InProgress(blockID):
				continue

			case len(nodes) > State.ReplicationFactor:
				log.Println("Block '" + blockID + "' is over-replicated")
				var deleteFrom []NodeID
				nodesByUtilization := State.MostUsedNodes()
				for _, nodeID := range nodesByUtilization {
					if len(nodes) - len(deleteFrom) <= State.ReplicationFactor {
						break
					}
					if nodes[nodeID] {
						deleteFrom = append(deleteFrom, nodeID)
					}
				}
				log.Printf("Deleting from: %v", deleteFrom)
				State.deletionIntents.Add(blockID, deleteFrom)

			case len(nodes) < State.ReplicationFactor:
				log.Println("Block '" + blockID + "' is under-replicated!")
				var forwardTo []NodeID
				nodesByUtilization := State.LeastUsedNodes()
				for _, nodeID := range nodesByUtilization {
					if len(forwardTo) + len(nodes) >= State.ReplicationFactor {
						break
					}
					if ! nodes[nodeID] {
						forwardTo = append(forwardTo, nodeID)
					}
				}
				log.Printf("Replicating to: %v", forwardTo)
				var availableFrom []NodeID
				for n, _ := range nodes {
					availableFrom = append(availableFrom, n)
				}
				State.replicationIntents.Add(blockID, availableFrom, forwardTo)
			}
		}
		State.mutex.Unlock()

		time.Sleep(2 * time.Second)
	}
}