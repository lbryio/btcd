package cmd

import (
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/claimtrie/node"
)

var status = map[node.Status]string{
	node.Accepted:    "Accepted",
	node.Activated:   "Activated",
	node.Deactivated: "Deactivated",
}

func changeName(c node.ChangeType) string {
	switch c { // can't this be done via reflection?
	case node.AddClaim:
		return "AddClaim"
	case node.SpendClaim:
		return "SpendClaim"
	case node.UpdateClaim:
		return "UpdateClaim"
	case node.AddSupport:
		return "AddSupport"
	case node.SpendSupport:
		return "SpendSupport"
	}
	return "Unknown"
}

func showChange(chg node.Change) {
	fmt.Printf(">>> Height: %6d: %s for %04s, %d, %s\n",
		chg.Height, changeName(chg.Type), chg.ClaimID.Hex(), chg.Amount, chg.OutPoint)
}

func showClaim(c *node.Claim, n *node.Node) {
	mark := " "
	if c == n.BestClaim {
		mark = "*"
	}

	fmt.Printf("%s  C  ID: %s, TXO: %s\n   %5d/%-5d, Status: %9s, Amount: %15d, Effective Amount: %15d\n",
		mark, c.ClaimID.Hex(), c.OutPoint, c.AcceptedAt, c.ActiveAt, status[c.Status], c.Amount, c.EffectiveAmount(n.Supports))
}

func showSupport(c *node.Claim) {
	fmt.Printf("    S id: %s, op: %s, %5d/%-5d, %9s, amt: %15d\n",
		c.ClaimID.Hex(), c.OutPoint, c.AcceptedAt, c.ActiveAt, status[c.Status], c.Amount)
}

func showNode(n *node.Node) {

	fmt.Printf("%s\n", strings.Repeat("-", 200))
	fmt.Printf("Last Node Takeover: %d\n\n", n.TakenOverAt)
	n.SortClaims()
	for _, c := range n.Claims {
		showClaim(c, n)
		for _, s := range n.Supports {
			if s.ClaimID != c.ClaimID {
				continue
			}
			showSupport(s)
		}
	}
	fmt.Printf("\n\n")
}
