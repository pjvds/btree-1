package btree

import (
	"sync"
	"bytes"
	"code.google.com/p/goprotobuf/proto"
)

const  NODEIDBASE = 1<<18

type Btree struct {
	info *BtreeMetaData
	nodes []TreeNode
	sync.RWMutex
}
type  Leaf struct {
	LeafMetaData
	sync.RWMutex
}
type Node struct {
	NodeMetaData
	sync.RWMutex
}
type TreeNode interface {
	insert(record *RecordMetaData, tree *Btree) bool
	delete(key []byte, tree *Btree) bool
	update(recode *RecordMetaData, tree *Btree) bool
	search(key []byte, tree *Btree) []byte
}

func NewBtree() *Btree {
	tree := new(Btree)
	tree.nodes = make([]TreeNode, 1<<20) //26 -> 1G mem
	tree.info = &BtreeMetaData{
	Size: proto.Uint32(32),
	LeafMax:  proto.Int32(NODEIDBASE-1),
	NodeMax: proto.Int32(1<<19),
	LeafCount: proto.Int32(0),
	NodeCount:  proto.Int32(0),
	LastLeaf: proto.Int32(-1),
	LastNode: proto.Int32(NODEIDBASE-1),
	FirstLeaf: proto.Int32(0),
	}
	tree.info.Root =  proto.Int32(tree.newleaf())
	return tree
}

func NewBtreeSize(size uint32) *Btree {
	tree := new(Btree)
	tree.nodes = make([]TreeNode, 1<<20)
	tree.info = &BtreeMetaData{
	Size: proto.Uint32(size),
	LeafMax:  proto.Int32(NODEIDBASE-1),
	NodeMax: proto.Int32(1<<19),
	LeafCount: proto.Int32(0),
	NodeCount:  proto.Int32(0),
	LastLeaf: proto.Int32(-1),
	LastNode: proto.Int32(NODEIDBASE-1),
	FirstLeaf: proto.Int32(0),
	}
	tree.info.Root =  proto.Int32(tree.newleaf())
	return tree
}

func (T *Btree) Insert(record *RecordMetaData, rst chan bool) {
	rst <- insert(T.nodes[*T.info.Root], record, T)
}

func (T *Btree) Delete(key []byte, rst chan bool) {
	rst <- delete(T.nodes[*T.info.Root], key, T)
}

func (T *Btree) Search(key []byte, rst chan []byte) {
	rst <- search(T.nodes[*T.info.Root], key, T)
}

func (T *Btree) Update(record *RecordMetaData, rst chan bool) {
	rst <- update(T.nodes[*T.info.Root], record, T)
}
/*
 * alloc leaf/node
 */
func (T *Btree) newleaf() int32 {
	T.Lock()
	defer T.Unlock()
	if *T.info.LastLeaf >= *T.info.LeafMax && len(T.nodes) < 1<<24 {
		T.nodes = append(T.nodes[:*T.info.LastLeaf], append(make([]TreeNode, *T.info.LeafCount), T.nodes[*T.info.LastLeaf:]...)...)
		*T.info.LeafMax = *T.info.LeafCount + *T.info.LastLeaf
	}
	*T.info.LastLeaf ++
	*T.info.LeafCount ++
	leaf := new(Leaf)
	leaf.Id = proto.Int32(*T.info.LastLeaf)
	leaf.Removed = proto.Bool(false)
	T.nodes[*T.info.LastLeaf] = leaf
	return *T.info.LastLeaf
}
func (T *Btree) newnode() int32 {
	T.Lock()
	defer T.Unlock()
	if *T.info.LastNode >= *T.info.NodeMax && len(T.nodes) < 1<<24 {
		T.nodes = append(T.nodes[:*T.info.LastNode], append(make([]TreeNode, *T.info.NodeCount), T.nodes[*T.info.LastNode:]...)...)
		*T.info.NodeMax = *T.info.NodeCount + *T.info.LastNode
	}
	*T.info.LastNode ++
	*T.info.NodeCount ++
	node := new(Node)
	node.Id = proto.Int32(*T.info.LastNode)
	node.Removed = proto.Bool(false)
	T.nodes[*T.info.LastNode] = node
	return *T.info.LastNode
}
/*
 * Insert
 */
func insert(treenode TreeNode, record *RecordMetaData, tree *Btree) bool {
	if node, ok := treenode.(*Node); ok {
		return node.insert(record, tree)

	}
	if leaf, ok := treenode.(*Leaf); ok {
		return leaf.insert(record, tree)
	}
	return false
}
func (N *Node) insert(record *RecordMetaData, tree *Btree) bool {
	N.Lock()
	defer N.Unlock()
	index := N.locate(record.Key)
	return insert(tree.nodes[N.Childrens[index]], record, tree)
}
func (L *Leaf) insert(record *RecordMetaData, tree *Btree) bool {
	L.Lock()
	defer L.Unlock()
	index := L.locate(record.Key)
	if index > 0 {
		if bytes.Compare(L.Records[index-1].Key, record.Key) == 0 {
			return false
		}
	}
	L.Records = append(L.Records[:index], append([]*RecordMetaData{record}, L.Records[index:]...)...)
	if uint32(len(L.Records)) > *tree.info.Size {
		L.split(tree)
	}
	return true
}
/*
 * Search
 */
func search(treenode TreeNode, key []byte, tree *Btree) []byte {
	if node, ok := treenode.(*Node); ok {
		return node.search(key, tree)
	}
	if leaf, ok := treenode.(*Leaf); ok {
		return leaf.search(key, tree)
	}
	return nil

}
func (N *Node) search(key []byte, tree *Btree) []byte {
	N.RLock()
	defer N.RUnlock()
	index := N.locate(key)
	return search(tree.nodes[N.Childrens[index]], key, tree)
}
func (L *Leaf) search(key []byte, tree *Btree) []byte {
	L.RLock()
	defer L.RUnlock()
	index := L.locate(key) - 1
	if index >= 0 {
		if bytes.Compare(L.Records[index].Key, key) == 0 {
			return L.Records[index].Value
		}
	}
	return nil
}
/*
 * Delete
 */
func delete(treenode TreeNode, key []byte, tree *Btree) bool {
	if node, ok := treenode.(*Node); ok {
		return node.delete(key, tree)
	}
	if leaf, ok := treenode.(*Leaf); ok {
		return leaf.delete(key, tree)
	}
	return false
}
func (N *Node) delete(key []byte, tree *Btree) bool {
	N.Lock()
	defer N.Unlock()
	index := N.locate(key)
	return delete(tree.nodes[N.Childrens[index]], key, tree)
}
func (L *Leaf) delete(key []byte, tree *Btree) bool {
	L.Lock()
	defer L.Unlock()
	var deleted bool
	index := L.locate(key) -1
	if index >= 0 {
		if bytes.Compare(L.Records[index].Key, key) == 0 {
			deleted = true
		}
	}
	if deleted {
		L.Records = append(L.Records[:index],L.Records[index+1:]...)
		if index == 0 && len(L.Records) > 0 {
			if tree.info.Root != L.Id {
				replace(key, L.Records[0].Key, *L.Father, tree)
			}
		}
		if L.Id != tree.info.Root {
			node := tree.nodes[*L.Father]
			if n, ok := node.(*Node); ok {
				merge(key, n, tree)
			}
		}
		return true
	}
	return false
}
/*
 * Update
 */
func update(treenode TreeNode, record *RecordMetaData, tree *Btree) bool {
	if node, ok := treenode.(*Node); ok {
		return node.update(record, tree)
	}
	if leaf, ok := treenode.(*Leaf); ok {
		return leaf.update(record, tree)
	}
	return false
}
func (N *Node) update(record *RecordMetaData, tree *Btree) bool {
	N.Lock()
	defer N.Unlock()
	index := N.locate(record.Key)
	return tree.nodes[N.Childrens[index]].update(record, tree)
}

func (L *Leaf) update(record *RecordMetaData, tree *Btree) bool {
	L.Lock()
	defer L.Unlock()
	index := L.locate(record.Key) - 1
	if index >= 0 {
		if bytes.Compare(L.Records[index].Key, record.Key) == 0 {
			L.Records[index].Value = record.Value
			return true
		}
	}
	return false
}
/*
 * Split
 */
func (L *Leaf) split(tree *Btree) {
	newleaf := get_leaf(tree.newleaf(), tree)
	newleaf.Records = make([]*RecordMetaData, len(L.Records[*tree.info.Size/2:]))
	copy(newleaf.Records, L.Records[*tree.info.Size/2:])
	L.Records = L.Records[:*tree.info.Size/2]
	L.Next = newleaf.Id
	newleaf.Prev = L.Id
	if *tree.info.NodeCount != 0 {
		tnode := get_node(*L.Father, tree)
		newleaf.Father = L.Father
		tnode.insert_once(newleaf.Records[0].Key, *L.Id, *newleaf.Id, tree)
	} else {
		tnode := get_node(tree.newnode(), tree)
		tnode.insert_once(newleaf.Records[0].Key, *L.Id, *newleaf.Id, tree)
		L.Father = tnode.Id
		newleaf.Father = L.Father
		tree.Lock()
		tree.info.Root = tnode.Id
		tree.Unlock()
	}
}
func (N *Node) split(tree *Btree) {
	newnode := get_node(tree.newnode(), tree)
	key := N.Keys[*tree.info.Size/2]
	newnode.Keys = make([][]byte, len(N.Keys[*tree.info.Size/2+1:]))
	copy(newnode.Keys, N.Keys[*tree.info.Size/2+1:])
	N.Keys = N.Keys[:*tree.info.Size/2]
	newnode.Childrens = make([]int32, len(N.Childrens[*tree.info.Size/2+1:]))
	copy(newnode.Childrens, N.Childrens[*tree.info.Size/2+1:])
	N.Childrens = N.Childrens[:*tree.info.Size/2+1]
	for l := 0; l < len(newnode.Childrens); l++ {
		set_father(tree.nodes[newnode.Childrens[l]], newnode.Id)
	}
	if N.Id == tree.info.Root {
		tnode := get_node(tree.newnode(), tree)
		N.Father = tnode.Id
		newnode.Father = N.Father
		tnode.insert_once(key, *N.Id, *newnode.Id, tree)
		tree.Lock()
		tree.info.Root = tnode.Id
		tree.Unlock()
	} else {
		newnode.Father = N.Father
		tnode := get_node(*N.Father, tree)
		tnode.insert_once(key, *N.Id, *newnode.Id, tree)
	}

}
/*
 * insert key into tree node
 */
func (N *Node) insert_once(key []byte, left_id int32, right_id int32, tree *Btree) {
	index := N.locate(key)
	if len(N.Keys) == 0 {
		N.Childrens = append([]int32{left_id}, right_id)
	} else {
		N.Childrens = append(N.Childrens[:index+1], append([]int32{right_id}, N.Childrens[index+1:]...)...)
	}
	N.Keys = append(N.Keys[:index], append([][]byte{key}, N.Keys[index:]...)...)
	if len(N.Keys) > int(*tree.info.Size) {
		N.split(tree)
	}
}
/*
 * Replace key in node
 */
func replace(oldkey []byte, newkey []byte, id int32, tree *Btree) {
	node := get_node(id, tree)
	if node != nil {
		index := node.locate(oldkey) - 1
		if index >= 0 {
			if bytes.Compare(node.Keys[index], oldkey) == 0 {
				node.Keys[index] = newkey
				return
			}
		}
		if tree.info.Root != node.Id {
			replace(oldkey, newkey, *node.Father, tree)
		}
	}
}
/*
 * merge leaf/node
 */
func merge(key []byte, node *Node, tree *Btree) {
	index := node.locate(key)
	if index == 0 {
		index = 1
	}
	if get_node(node.Childrens[0], tree) != nil {
		node.mergenode(node.Childrens[index-1], node.Childrens[index], index-1, tree)
	} else {
		removed_key := node.Keys[0]
		node.mergeleaf(node.Childrens[index-1], node.Childrens[index], index-1, tree)
		if proto.GetBool(node.Removed) == false {
			replace(key, removed_key, *node.Id, tree)
		}
	}
}

func (N *Node) mergeleaf(left_id int32, right_id int32, index int, tree *Btree) {
	left := get_leaf(left_id, tree)
	right := get_leaf(right_id, tree)
	if (len(left.Records) + len(right.Records)) > int(*tree.info.Size) {
		return
	}
	if index == len(N.Keys) {
		N.Childrens = N.Childrens[:index]
		N.Keys = N.Keys[:index-1]
	} else {
		N.Childrens = append(N.Childrens[:index+1], N.Childrens[index+2:]...)
		N.Keys = append(N.Keys[:index],N.Keys[index+1:]...)
	}
	left.Records = append(left.Records, right.Records...)
	right.Records = right.Records[:0]
	left.Next = right.Next
	if right.Next != nil {
		nextleaf := get_leaf(*right.Next, tree)
		nextleaf.Prev = left.Id
	}
	remove(tree.nodes[*right.Id])
	tree.Lock()
	*tree.info.LeafCount --
	tree.Unlock()
	if N.Id != tree.info.Root {
		node := get_node(*N.Father, tree)
		merge(left.Records[0].Key, node, tree)
	} else {
		tree.Lock()
		if len(N.Keys) == 0 {
			remove(tree.nodes[*tree.info.Root])
			tree.info.Root = left.Id
			*tree.info.NodeCount --
		}
		tree.Unlock()
	}
}
func (N *Node) mergenode(left_id int32, right_id int32, index int, tree *Btree) {
	left := get_node(left_id, tree)
	right := get_node(right_id, tree)
	if len(left.Keys) + len(right.Keys) >  int(*tree.info.Size) {
		return
	}
	key := N.Keys[0]
	for l := 0; l < len(right.Childrens); l++ {
		set_father(tree.nodes[right.Childrens[l]], left.Id)
	}
	left.Keys = append(left.Keys, append([][]byte{N.Keys[index]}, right.Keys...)...)
	left.Childrens = append(left.Childrens, right.Childrens...)
	right.Keys = right.Keys[:0]
	right.Childrens = right.Childrens[:0]
	N.Keys = append(N.Keys[:index],N.Keys[index+1:]...)
	N.Childrens = append(N.Childrens[:index+1], N.Childrens[index+2:]...)
	remove(tree.nodes[*right.Id])
	if len(left.Keys) > int(*tree.info.Size) {
		left.split(tree)
	} else {
		if N.Id != tree.info.Root {
			node := get_node(*N.Father, tree)
			merge(key, node, tree)
		} else {
			tree.Lock()
			if len(N.Keys) == 0 {
				remove(tree.nodes[*tree.info.Root])
				*tree.info.NodeCount --
				tree.info.Root = left.Id
			}
			tree.Unlock()
		}
	}
}

func remove(treenode TreeNode) {
	if node, ok := treenode.(*Node); ok {
		node.Removed = proto.Bool(true)
	}
	if leaf, ok := treenode.(*Leaf); ok {
		leaf.Removed = proto.Bool(true)
	}
}
func get_father(treenode TreeNode) int32 {
	var id int32
	if node, ok := treenode.(*Node); ok {
		id = *node.Father
	}
	if leaf, ok := treenode.(*Leaf); ok {
		id = *leaf.Father
	}
	return id
}
func set_father(treenode TreeNode, id *int32) {
	if node, ok := treenode.(*Node); ok {
		node.Father = id
	}
	if leaf, ok := treenode.(*Leaf); ok {
		leaf.Father = id
	}
}
func get_node(id int32, tree *Btree) (*Node) {
	if node, ok := tree.nodes[id].(*Node); ok {
		return node
	}
	return nil
}
func get_leaf(id int32, tree *Btree) (*Leaf) {
	if leaf, ok := tree.nodes[id].(*Leaf); ok {
		return leaf
	}
	return nil
}
func (N *Node) locate(key []byte) (int) {
	i := 0
	size := len(N.Keys)
	for {
		mid := (i+size)/2
		if i == size {
			break
		}
		if bytes.Compare(N.Keys[mid], key) <= 0 {
			i = mid + 1
		} else {
			size = mid
		}
	}
	return i
}
func (L *Leaf) locate(key []byte) (int) {
	i := 0
	size := len(L.Records)
	for {
		mid := (i+size)/2
		if i == size {
			break
		}
		if bytes.Compare(L.Records[mid].Key, key) <= 0 {
			i = mid + 1
		} else {
			size = mid
		}
	}
	return i
}
