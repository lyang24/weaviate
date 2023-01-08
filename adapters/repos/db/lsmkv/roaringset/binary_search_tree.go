package roaringset

import (
	"bytes"

	"github.com/dgraph-io/sroar"
	"github.com/semi-technologies/weaviate/adapters/repos/db/lsmkv/ent"
	"github.com/semi-technologies/weaviate/adapters/repos/db/lsmkv/rbtree"
)

type BinarySearchTree struct {
	root *BinarySearchNode
}

type roaringSet struct {
	additions *sroar.Bitmap
	deletions *sroar.Bitmap
}

type Insert struct {
	Additions []uint64
	Deletions []uint64
}

func newRoaringSet() roaringSet {
	return roaringSet{
		additions: sroar.NewBitmap(),
		deletions: sroar.NewBitmap(),
	}
}

func newSroarFromList(list []uint64) *sroar.Bitmap {
	bm := sroar.NewBitmap()
	bm.SetMany(list)
	return bm
}

func (t *BinarySearchTree) Insert(key []byte, values Insert) {
	if t.root == nil {
		t.root = &BinarySearchNode{
			Key: key,
			Value: BitmapLayer{
				Additions: newSroarFromList(values.Additions),
				Deletions: newSroarFromList(values.Deletions),
			},
			colourIsRed: false, // root node is always black
		}
		return
	}

	if newRoot := t.root.insert(key, values); newRoot != nil {
		t.root = newRoot
	}
	t.root.colourIsRed = false // Can be flipped in the process of balancing, but root is always black
}

func (t *BinarySearchTree) Get(key []byte) (*BitmapLayer, error) {
	if t.root == nil {
		return nil, ent.NotFound
	}

	return t.root.get(key)
}

func (t *BinarySearchTree) FlattenInOrder() []*BinarySearchNode {
	if t.root == nil {
		return nil
	}

	return t.root.flattenInOrder()
}

type BinarySearchNode struct {
	Key         []byte
	Value       BitmapLayer
	left        *BinarySearchNode
	right       *BinarySearchNode
	parent      *BinarySearchNode
	colourIsRed bool
}

func (n *BinarySearchNode) Parent() rbtree.Node {
	if n == nil {
		return nil
	}
	return n.parent
}

func (n *BinarySearchNode) SetParent(parent rbtree.Node) {
	if n == nil {
		addNewSearchNodeRoaringSetReceiver(&n)
	}

	if parent == nil {
		n.parent = nil
		return
	}

	n.parent = parent.(*BinarySearchNode)
}

func (n *BinarySearchNode) Left() rbtree.Node {
	if n == nil {
		return nil
	}
	return n.left
}

func (n *BinarySearchNode) SetLeft(left rbtree.Node) {
	if n == nil {
		addNewSearchNodeRoaringSetReceiver(&n)
	}

	if left == nil {
		n.left = nil
		return
	}

	n.left = left.(*BinarySearchNode)
}

func (n *BinarySearchNode) Right() rbtree.Node {
	if n == nil {
		return nil
	}
	return n.right
}

func (n *BinarySearchNode) SetRight(right rbtree.Node) {
	if n == nil {
		addNewSearchNodeRoaringSetReceiver(&n)
	}

	if right == nil {
		n.right = nil
		return
	}

	n.right = right.(*BinarySearchNode)
}

func (n *BinarySearchNode) IsRed() bool {
	if n == nil {
		return false
	}
	return n.colourIsRed
}

func (n *BinarySearchNode) SetRed(isRed bool) {
	n.colourIsRed = isRed
}

func (n *BinarySearchNode) IsNil() bool {
	return n == nil
}

func addNewSearchNodeRoaringSetReceiver(nodePtr **BinarySearchNode) {
	*nodePtr = &BinarySearchNode{}
}

func (n *BinarySearchNode) insert(key []byte, values Insert) *BinarySearchNode {
	if bytes.Equal(key, n.Key) {
		// Merging the new additions and deletions into the existing ones is a
		// four-step process:
		//
		// 1. make sure anything that's added is not part of the deleted list, in
		//    case it was previously deleted
		// 2. actually add the new entries to additions
		// 3. make sure anything that's deleted is not part of the additions list,
		//    in case it was recently added
		// 4. actually add the new entries to deletions (this step is vital in case
		//    a delete points to an entry of a previous segment that's not added in
		//    this memtable)
		for _, x := range values.Additions {
			n.Value.Deletions.Remove(x)
			n.Value.Additions.Set(x)
		}

		for _, x := range values.Deletions {
			n.Value.Additions.Remove(x)
			n.Value.Deletions.Set(x)
		}

		return nil
	}

	if bytes.Compare(key, n.Key) < 0 {
		if n.left != nil {
			return n.left.insert(key, values)
		} else {
			n.left = &BinarySearchNode{
				Key: key,
				Value: BitmapLayer{
					Additions: newSroarFromList(values.Additions),
					Deletions: newSroarFromList(values.Deletions),
				},
				parent:      n,
				colourIsRed: true,
			}
			return BinarySearchNodeFromRB(rbtree.Rebalance(n.left))
		}
	} else {
		if n.right != nil {
			return n.right.insert(key, values)
		} else {
			n.right = &BinarySearchNode{
				Key: key,
				Value: BitmapLayer{
					Additions: newSroarFromList(values.Additions),
					Deletions: newSroarFromList(values.Deletions),
				},
				parent:      n,
				colourIsRed: true,
			}
			return BinarySearchNodeFromRB(rbtree.Rebalance(n.right))
		}
	}
}

func (n *BinarySearchNode) get(key []byte) (*BitmapLayer, error) {
	if bytes.Equal(n.Key, key) {
		return &n.Value, nil
	}

	if bytes.Compare(key, n.Key) < 0 {
		if n.left == nil {
			return nil, ent.NotFound
		}

		return n.left.get(key)
	} else {
		if n.right == nil {
			return nil, ent.NotFound
		}

		return n.right.get(key)
	}
}

func BinarySearchNodeFromRB(rbNode rbtree.Node) (bsNode *BinarySearchNode) {
	if rbNode == nil {
		bsNode = nil
		return
	}
	bsNode = rbNode.(*BinarySearchNode)
	return
}

func (n *BinarySearchNode) flattenInOrder() []*BinarySearchNode {
	var left []*BinarySearchNode
	var right []*BinarySearchNode

	if n.left != nil {
		left = n.left.flattenInOrder()
	}

	if n.right != nil {
		right = n.right.flattenInOrder()
	}

	right = append([]*BinarySearchNode{n}, right...)
	return append(left, right...)
}
