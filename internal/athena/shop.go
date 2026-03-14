/* Athena - A server for Attorney Online 2 written in Go
Copyright (C) 2022 MangosArentLiterature <mango@transmenace.dev>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>. */

package athena

import (
	"fmt"
	"strings"

	"github.com/MangosArentLiterature/Athena/internal/db"
)

// shopItemKind classifies each item in the shop.
type shopItemKind int

const (
	shopKindTag  shopItemKind = iota // cosmetic tag shown in /gas and /players
	shopKindPass                     // permanent perk (job cooldown / bonus)
)

// shopItem describes a single purchasable item.
type shopItem struct {
	id                string
	name              string
	kind              shopItemKind
	price             int64
	description       string
	cooldownReduction int64 // seconds to shave off ALL job cooldowns (passes only)
	jobBonus          int64 // extra chips awarded per job completion (passes only)
}

// shopItems is the master catalog.  All entries are displayed by /shop.
var shopItems = []shopItem{
	// ── Cosmetic Tags (30 total, gambling-themed, ascending price) ────────────

	{id: "tag_gambler", name: "Gambler", kind: shopKindTag, price: 1_000,
		description: "Show the [Gambler] tag — the first step on every high roller's journey."},
	{id: "tag_lucky", name: "Lucky", kind: shopKindTag, price: 2_500,
		description: "Show the [Lucky] tag — fortune smiles on you."},
	{id: "tag_risk_taker", name: "Risk Taker", kind: shopKindTag, price: 5_000,
		description: "Show the [Risk Taker] tag — you live on the edge."},
	{id: "tag_card_shark", name: "Card Shark", kind: shopKindTag, price: 7_500,
		description: "Show the [Card Shark] tag — the cards bend to your will."},
	{id: "tag_high_roller", name: "High Roller", kind: shopKindTag, price: 10_000,
		description: "Show the [High Roller] tag — big bets, bigger wins."},
	{id: "tag_patreon", name: "Patreon", kind: shopKindTag, price: 15_000,
		description: "Show the [Patreon] tag — a proud supporter of the casino."},
	{id: "tag_chip_collector", name: "Chip Collector", kind: shopKindTag, price: 25_000,
		description: "Show the [Chip Collector] tag — you have amassed a serious stack."},
	{id: "tag_jackpot", name: "Jackpot", kind: shopKindTag, price: 35_000,
		description: "Show the [Jackpot] tag — you've hit it big."},
	{id: "tag_casino_regular", name: "Casino Regular", kind: shopKindTag, price: 50_000,
		description: "Show the [Casino Regular] tag — the staff know your name."},
	{id: "tag_hustler", name: "Hustler", kind: shopKindTag, price: 75_000,
		description: "Show the [Hustler] tag — always working the angles."},
	{id: "tag_all_in", name: "All In", kind: shopKindTag, price: 100_000,
		description: "Show the [All In] tag — no half measures."},
	{id: "tag_whale", name: "Whale", kind: shopKindTag, price: 150_000,
		description: "Show the [Whale] tag — you move markets with your bets."},
	{id: "tag_bluffer", name: "Bluffer", kind: shopKindTag, price: 200_000,
		description: "Show the [Bluffer] tag — your poker face is legendary."},
	{id: "tag_odds_defier", name: "Odds Defier", kind: shopKindTag, price: 300_000,
		description: "Show the [Odds Defier] tag — statistics fear you."},
	{id: "tag_dealer", name: "Dealer", kind: shopKindTag, price: 400_000,
		description: "Show the [Dealer] tag — you know where every card is."},
	{id: "tag_ace", name: "Ace", kind: shopKindTag, price: 500_000,
		description: "Show the [Ace] tag — always the highest card in the deck."},
	{id: "tag_double_down", name: "Double Down", kind: shopKindTag, price: 600_000,
		description: "Show the [Double Down] tag — you never back away from a good hand."},
	{id: "tag_full_house", name: "Full House", kind: shopKindTag, price: 750_000,
		description: "Show the [Full House] tag — a powerful hand, a powerful player."},
	{id: "tag_flush", name: "Flush", kind: shopKindTag, price: 900_000,
		description: "Show the [Flush] tag — all the right suits, all the right moves."},
	{id: "tag_lucky_charm", name: "Lucky Charm", kind: shopKindTag, price: 1_000_000,
		description: "Show the [Lucky Charm] tag — a million chips of proof."},
	{id: "tag_bankroll", name: "Bankroll", kind: shopKindTag, price: 1_250_000,
		description: "Show the [Bankroll] tag — your wealth speaks for itself."},
	{id: "tag_fortune", name: "Fortune's Fave", kind: shopKindTag, price: 1_500_000,
		description: "Show the [Fortune's Fave] tag — the universe bends your way."},
	{id: "tag_degenerate", name: "Degenerate", kind: shopKindTag, price: 2_000_000,
		description: "Show the [Degenerate] tag — you wear it as a badge of honour."},
	{id: "tag_the_house", name: "The House", kind: shopKindTag, price: 2_500_000,
		description: "Show the [The House] tag — the house always wins, and you are the house."},
	{id: "tag_legendary", name: "Legendary", kind: shopKindTag, price: 3_000_000,
		description: "Show the [Legendary] tag — stories are told about players like you."},
	{id: "tag_casino_royale", name: "Casino Royale", kind: shopKindTag, price: 4_000_000,
		description: "Show the [Casino Royale] tag — the ultimate casino experience."},
	{id: "tag_diamond", name: "Diamond", kind: shopKindTag, price: 5_000_000,
		description: "Show the [Diamond] tag — rare, brilliant, and impossibly valuable."},
	{id: "tag_mythic", name: "Mythic", kind: shopKindTag, price: 6_000_000,
		description: "Show the [Mythic] tag — spoken of in hushed reverence."},
	{id: "tag_godlike", name: "Godlike", kind: shopKindTag, price: 7_500_000,
		description: "Show the [Godlike] tag — mortals cannot comprehend your wins."},
	{id: "tag_infinite", name: "Infinite", kind: shopKindTag, price: 10_000_000,
		description: "Show the [Infinite] tag — the ultimate flex. You have truly seen it all."},

	// ── Job Passes (cooldown reduction — all stack, up to 50 min total reduction) ─

	{id: "pass_quick", name: "Quick Worker Pass", kind: shopKindPass, price: 10_000,
		cooldownReduction: 5 * 60,
		description: "Permanently reduces ALL job cooldowns by 5 minutes."},
	{id: "pass_speedy", name: "Speedy Pass", kind: shopKindPass, price: 50_000,
		cooldownReduction: 10 * 60,
		description: "Permanently reduces ALL job cooldowns by an additional 10 minutes."},
	{id: "pass_turbo", name: "Turbo Pass", kind: shopKindPass, price: 150_000,
		cooldownReduction: 15 * 60,
		description: "Permanently reduces ALL job cooldowns by an additional 15 minutes. (Stack with Quick & Speedy for up to 30 min off!)"},
	{id: "pass_lightning", name: "Lightning Pass", kind: shopKindPass, price: 500_000,
		cooldownReduction: 20 * 60,
		description: "Permanently reduces ALL job cooldowns by an additional 20 minutes. (Full stack = 50 min off, minimum 5 min cooldown.)"},

	// ── Job Passes (reward bonus — all stack) ─────────────────────────────────

	{id: "pass_bonus", name: "Bonus Chip Pass", kind: shopKindPass, price: 25_000,
		jobBonus:    1,
		description: "Permanently earn +1 extra chip on every job completion."},
	{id: "pass_extra", name: "Extra Chip Pass", kind: shopKindPass, price: 100_000,
		jobBonus:    2,
		description: "Permanently earn +2 extra chips on every job completion."},
	{id: "pass_lucky_find", name: "Lucky Find Pass", kind: shopKindPass, price: 400_000,
		jobBonus:    3,
		description: "Permanently earn +3 extra chips on every job completion."},
	{id: "pass_jackpot_seeker", name: "Jackpot Seeker Pass", kind: shopKindPass, price: 1_000_000,
		jobBonus:    5,
		description: "Permanently earn +5 extra chips on every job completion. (Full stack = +11 chips per job!)"},
}

// shopItemByID returns the item with the given ID, or (shopItem{}, false) if not found.
func shopItemByID(id string) (shopItem, bool) {
	for _, it := range shopItems {
		if it.id == id {
			return it, true
		}
	}
	return shopItem{}, false
}

// formatTagDisplay returns the bracketed tag label, e.g. "[High Roller]".
// Returns "" when tagID is empty or unknown.
func formatTagDisplay(tagID string) string {
	if tagID == "" {
		return ""
	}
	if it, ok := shopItemByID(tagID); ok && it.kind == shopKindTag {
		return "[" + it.name + "]"
	}
	return ""
}

// getPlayerCooldownReduction returns the total job-cooldown reduction in seconds
// for the given IPID, summing up all purchased cooldown passes.
func getPlayerCooldownReduction(ipid string) int64 {
	items, err := db.GetPlayerShopItems(ipid)
	if err != nil || len(items) == 0 {
		return 0
	}
	var total int64
	for _, id := range items {
		if it, ok := shopItemByID(id); ok {
			total += it.cooldownReduction
		}
	}
	return total
}

// getPlayerJobBonus returns the total extra chips per job completion for the
// given IPID, summing up all purchased job-bonus passes.
func getPlayerJobBonus(ipid string) int64 {
	items, err := db.GetPlayerShopItems(ipid)
	if err != nil || len(items) == 0 {
		return 0
	}
	var total int64
	for _, id := range items {
		if it, ok := shopItemByID(id); ok {
			total += it.jobBonus
		}
	}
	return total
}

// ── /shop command ─────────────────────────────────────────────────────────────

// cmdShop handles the /shop command.
//
//	/shop              — show the catalog
//	/shop buy <id>     — purchase an item
//	/shop items        — list your owned items
func cmdShop(client *Client, args []string, _ string) {
	if !config.EnableCasino {
		client.SendServerMessage("The casino (and shop) is not enabled on this server.")
		return
	}

	if len(args) == 0 {
		printShopCatalog(client)
		return
	}

	switch strings.ToLower(args[0]) {
	case "buy":
		if len(args) < 2 {
			client.SendServerMessage("Usage: /shop buy <item_id>  — use /shop to see available IDs.")
			return
		}
		shopBuy(client, args[1])
	case "items", "owned", "inventory":
		shopListOwned(client)
	default:
		client.SendServerMessage("Usage: /shop | /shop buy <item_id> | /shop items")
	}
}

func printShopCatalog(client *Client) {
	bal, _ := db.GetChipBalance(client.Ipid())

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n🛒 Nyathena Shop — Your balance: %d chips\n", bal))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("Spend your chips on permanent cosmetic tags and job perks!\n")
	sb.WriteString("All purchases are PERMANENT and linked to your account.\n")
	sb.WriteString("  Buy with:  /shop buy <item_id>\n")
	sb.WriteString("  Your items: /shop items\n")
	sb.WriteString("  Set tag:   /settag <tag_id>  or  /settag none  to remove\n\n")

	sb.WriteString("── 🏷️  Cosmetic Tags (visible in /gas and /players) ──\n")
	sb.WriteString(fmt.Sprintf("  %-26s  %-18s  %s\n", "Item ID", "Tag", "Price"))
	sb.WriteString("  " + strings.Repeat("─", 62) + "\n")
	for _, it := range shopItems {
		if it.kind != shopKindTag {
			continue
		}
		owned := ""
		if db.HasShopItem(client.Ipid(), it.id) {
			owned = " ✅"
		}
		sb.WriteString(fmt.Sprintf("  %-26s  [%-16s]  %d chips%s\n",
			it.id, it.name, it.price, owned))
	}

	sb.WriteString("\n── 💼 Job Passes (permanent bonuses for all jobs) ──\n")
	sb.WriteString(fmt.Sprintf("  %-26s  %-28s  %s\n", "Item ID", "Pass Name", "Price"))
	sb.WriteString("  " + strings.Repeat("─", 72) + "\n")
	for _, it := range shopItems {
		if it.kind != shopKindPass {
			continue
		}
		owned := ""
		if db.HasShopItem(client.Ipid(), it.id) {
			owned = " ✅"
		}
		var benefit string
		if it.cooldownReduction > 0 {
			benefit = fmt.Sprintf("-%d min cooldown", it.cooldownReduction/60)
		} else if it.jobBonus > 0 {
			benefit = fmt.Sprintf("+%d chip/job", it.jobBonus)
		}
		sb.WriteString(fmt.Sprintf("  %-26s  %-28s  %d chips%s\n  %-26s  %s\n",
			it.id, it.name, it.price, owned,
			"", benefit))
	}

	sb.WriteString("\n💡 Tags stack — buy them all for a richer display!\n")
	sb.WriteString("💡 Job passes stack — max cooldown reduction is 50 min (5 min floor per job).\n")
	sb.WriteString("💡 Bonus passes stack — max +11 chips per job when all 4 are owned.\n")
	client.SendServerMessage(sb.String())
}

func shopBuy(client *Client, itemID string) {
	it, ok := shopItemByID(itemID)
	if !ok {
		client.SendServerMessage(fmt.Sprintf("Unknown item '%v'. Use /shop to see all item IDs.", itemID))
		return
	}

	if db.HasShopItem(client.Ipid(), it.id) {
		client.SendServerMessage(fmt.Sprintf("You already own '%v'. Use /settag %v to equip it.", it.name, it.id))
		return
	}

	if err := db.PurchaseShopItem(client.Ipid(), it.id, it.price); err != nil {
		client.SendServerMessage(fmt.Sprintf("Purchase failed: %v", err))
		return
	}

	newBal, _ := db.GetChipBalance(client.Ipid())

	if it.kind == shopKindTag {
		// Auto-equip the tag on purchase.
		_ = db.SetActiveTag(client.Ipid(), it.id)
		client.SendServerMessage(fmt.Sprintf(
			"✅ Purchased [%v] tag for %d chips! It is now your active tag.\n"+
				"Use /settag <id> to switch tags, or /settag none to remove it.\n"+
				"Balance: %d chips", it.name, it.price, newBal))
	} else {
		var benefit string
		if it.cooldownReduction > 0 {
			benefit = fmt.Sprintf("job cooldowns reduced by %d min", it.cooldownReduction/60)
		} else if it.jobBonus > 0 {
			benefit = fmt.Sprintf("+%d chip bonus per job", it.jobBonus)
		}
		client.SendServerMessage(fmt.Sprintf(
			"✅ Purchased %v for %d chips!\n"+
				"Permanent benefit: %v\n"+
				"Balance: %d chips", it.name, it.price, benefit, newBal))
	}
}

func shopListOwned(client *Client) {
	items, err := db.GetPlayerShopItems(client.Ipid())
	if err != nil || len(items) == 0 {
		client.SendServerMessage("You don't own any shop items yet. Use /shop to browse the catalog.")
		return
	}

	activeTag := db.GetActiveTag(client.Ipid())
	var sb strings.Builder
	sb.WriteString("\n🛒 Your Shop Items\n")
	sb.WriteString("  " + strings.Repeat("─", 50) + "\n")

	var tags, passes []string
	for _, id := range items {
		if it, ok := shopItemByID(id); ok {
			if it.kind == shopKindTag {
				active := ""
				if id == activeTag {
					active = " ← active"
				}
				tags = append(tags, fmt.Sprintf("  [%-16s] — %s%s", it.name, it.id, active))
			} else {
				var benefit string
				if it.cooldownReduction > 0 {
					benefit = fmt.Sprintf("-%d min cooldown", it.cooldownReduction/60)
				} else if it.jobBonus > 0 {
					benefit = fmt.Sprintf("+%d chip/job", it.jobBonus)
				}
				passes = append(passes, fmt.Sprintf("  %-28s — %s", it.name, benefit))
			}
		}
	}

	if len(tags) > 0 {
		sb.WriteString("🏷️  Tags:\n")
		for _, t := range tags {
			sb.WriteString(t + "\n")
		}
		if activeTag == "" {
			sb.WriteString("  (no active tag — use /settag <id> to equip one)\n")
		}
	}
	if len(passes) > 0 {
		sb.WriteString("💼 Passes:\n")
		for _, p := range passes {
			sb.WriteString(p + "\n")
		}
		reduction := getPlayerCooldownReduction(client.Ipid())
		bonus := getPlayerJobBonus(client.Ipid())
		if reduction > 0 || bonus > 0 {
			sb.WriteString(fmt.Sprintf("  Total effect: -%d min cooldown | +%d chip/job\n",
				reduction/60, bonus))
		}
	}
	client.SendServerMessage(sb.String())
}

// ── /settag command ───────────────────────────────────────────────────────────

// cmdSetTag handles /settag <tag_id> or /settag none.
// Swaps the player's active cosmetic tag.
func cmdSetTag(client *Client, args []string, _ string) {
	if !config.EnableCasino {
		client.SendServerMessage("The casino (and shop) is not enabled on this server.")
		return
	}

	tagID := args[0]

	if strings.EqualFold(tagID, "none") || tagID == "" {
		if err := db.SetActiveTag(client.Ipid(), ""); err != nil {
			client.SendServerMessage("Failed to clear tag: " + err.Error())
			return
		}
		client.SendServerMessage("🏷️ Your active tag has been removed.")
		return
	}

	it, ok := shopItemByID(tagID)
	if !ok {
		client.SendServerMessage(fmt.Sprintf("Unknown tag '%v'. Use /shop items to see your owned tags.", tagID))
		return
	}
	if it.kind != shopKindTag {
		client.SendServerMessage(fmt.Sprintf("'%v' is not a tag — it's a pass. Tags are the cosmetic items shown in /gas.", it.name))
		return
	}
	if !db.HasShopItem(client.Ipid(), tagID) {
		client.SendServerMessage(fmt.Sprintf("You don't own [%v]. Purchase it first with: /shop buy %v", it.name, tagID))
		return
	}

	if err := db.SetActiveTag(client.Ipid(), tagID); err != nil {
		client.SendServerMessage("Failed to set tag: " + err.Error())
		return
	}
	client.SendServerMessage(fmt.Sprintf("🏷️ Active tag set to [%v]. It will now appear next to your name in /gas.", it.name))
}
