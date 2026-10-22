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

// Pebble Mode has its own cosmetic-only catalog. There are no passes, no
// cooldown reductions and no passive income: Pebbles buy tags, nothing else.

type pebbleShopItem struct {
	id          string
	name        string
	category    string
	price       int64
	description string
}

type pebbleShopCategory struct {
	key         string
	displayName string
	emoji       string
}

var pebbleShopCategories = []pebbleShopCategory{
	{"activity", "Activity", "🌿"},
	{"community", "Community", "🫶"},
}

var pebbleShopItems = []pebbleShopItem{
	{id: "tag_explorer", name: "Explorer", category: "activity", price: 3,
		description: "You went outside and looked at something."},
	{id: "tag_friendly", name: "Friendly", category: "community", price: 5,
		description: "You are nice to the new people."},
}

// Temporary event tags. These are never sold in the shop; they are granted by
// world events with an expiry timestamp.
const pebbleTempTagPrefix = "tag_temp_"

var pebbleTempTagNames = map[string]string{
	"tag_temp_salvager":  "Salvager",
	"tag_temp_castaway":  "Castaway",
	"tag_temp_picnicker": "Picnicker",
	"tag_temp_stargazer": "Stargazer",
	"tag_temp_trader":    "Trader",
}

var pebbleShopIndex map[string]pebbleShopItem

func init() {
	pebbleShopIndex = make(map[string]pebbleShopItem, len(pebbleShopItems))
	for _, it := range pebbleShopItems {
		pebbleShopIndex[it.id] = it
	}
}

func pebbleShopItemByID(id string) (pebbleShopItem, bool) {
	it, ok := pebbleShopIndex[id]
	return it, ok
}

func pebbleCategoryByKey(key string) (pebbleShopCategory, bool) {
	for _, c := range pebbleShopCategories {
		if c.key == key {
			return c, true
		}
	}
	return pebbleShopCategory{}, false
}

func lookupPebbleTagName(id string) (string, bool) {
	if name, ok := pebbleTempTagNames[id]; ok {
		return name, true
	}
	if it, ok := pebbleShopIndex[id]; ok {
		return it.name, true
	}
	return "", false
}

// isGamblingLegacyTag reports whether id is one of the legacy casino-themed
// shop tags that Pebble Mode intentionally excludes.
func isGamblingLegacyTag(id string) bool {
	it, ok := shopItemIndex[id]
	return ok && it.kind == shopKindTag && it.category == "gambling"
}

// cmdPebbleShop is the /shop implementation used in Pebble Mode.
func cmdPebbleShop(client *Client, args []string) {
	if len(args) == 0 {
		printPebbleShopOverview(client)
		return
	}
	sub := strings.ToLower(args[0])
	if sub == "classic" || sub == "free" || sub == "tags" {
		printPebbleClassicOverview(client)
		return
	}
	// Classic non-gambling categories are free to browse and equip in Pebble
	// Mode; they are not part of the Pebble-priced catalog.
	for _, cat := range shopCategories {
		if sub == cat.key && sub != "gambling" {
			printPebbleClassicCategory(client, cat)
			return
		}
	}
	if cat, ok := pebbleCategoryByKey(sub); ok {
		printPebbleShopCategory(client, cat)
		return
	}
	switch sub {
	case "buy":
		if len(args) < 2 {
			client.SendServerMessage("Usage: /shop buy <item_id>")
			return
		}
		pebbleShopBuy(client, args[1])
	case "items", "owned", "inventory":
		printPebbleOwned(client)
	case "passes", "passive", "upgrades":
		client.SendServerMessage("Pebble Mode has no upgrades. Pebbles buy cosmetic tags only — no cooldown passes, no passive income, no grinding.")
	case "help":
		printPebbleShopOverview(client)
	default:
		client.SendServerMessage("Usage: /shop | /shop classic | /shop activity | /shop community | /shop buy <item_id> | /shop items")
	}
}

func printPebbleShopOverview(client *Client) {
	bal, _ := db.GetPebbleBalance(client.Ipid())
	active := db.GetActiveTag(client.Ipid())
	activeDisplay := "(none)"
	if t := formatTagDisplay(active); t != "" {
		activeDisplay = t
	}
	owned, _ := db.GetPlayerShopItems(client.Ipid())
	ownedSet := make(map[string]struct{}, len(owned))
	for _, id := range owned {
		ownedSet[id] = struct{}{}
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n🪨 Pebble Shop — Balance: %d Pebbles | Active tag: %s\n", bal, activeDisplay))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("Cosmetic tags only. No upgrades, no grinding, no hierarchy.\n\n")
	sb.WriteString("🆓 All existing non-gambling tags are FREE: /shop classic\n\n")
	sb.WriteString("📂 Pebble-priced tags:\n")
	for _, cat := range pebbleShopCategories {
		ownedCount := 0
		for _, it := range pebbleShopItems {
			if it.category == cat.key {
				if _, ok := ownedSet[it.id]; ok {
					ownedCount++
				}
			}
		}
		count := 0
		for _, it := range pebbleShopItems {
			if it.category == cat.key {
				count++
			}
		}
		sb.WriteString(fmt.Sprintf("  %s %-10s — %d tags (owned: %d) → /shop %s\n",
			cat.emoji, cat.displayName, count, ownedCount, cat.key))
	}
	sb.WriteString("\n🛒 Commands:\n")
	sb.WriteString("  /shop buy <item_id> — buy a cosmetic tag with Pebbles\n")
	sb.WriteString("  /shop items         — list tags you own\n")
	sb.WriteString("  /settag <id>|none   — equip or remove your active tag\n")
	client.SendServerMessage(sb.String())
}

// printPebbleClassicOverview lists the existing non-gambling tag categories,
// all of which are free to equip in Pebble Mode.
func printPebbleClassicOverview(client *Client) {
	var sb strings.Builder
	sb.WriteString("\n🆓 Classic Tags — free for everyone in Pebble Mode\n")
	sb.WriteString("Use /settag <tag_id> to equip any of these. None of them cost Pebbles.\n\n")
	for _, cat := range shopCategories {
		if cat.key == "gambling" {
			continue
		}
		count := 0
		for _, it := range shopItems {
			if it.kind == shopKindTag && it.category == cat.key {
				count++
			}
		}
		sb.WriteString(fmt.Sprintf("  %s %-12s — %d tags → /shop %s\n", cat.emoji, cat.displayName, count, cat.key))
	}
	sb.WriteString("\nPaid Pebble tags: /shop activity and /shop community.\n")
	sb.WriteString("Gambling-themed tags are not used in Pebble Mode.")
	client.SendServerMessage(sb.String())
}

// printPebbleClassicCategory lists free classic tags with their ids.
func printPebbleClassicCategory(client *Client, cat shopCategoryInfo) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s %s Tags — FREE in Pebble Mode\n", cat.emoji, cat.displayName))
	sb.WriteString("Equip with /settag <item_id>.\n\n")
	for _, it := range shopItems {
		if it.kind != shopKindTag || it.category != cat.key {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %-26s — [%s]\n", it.id, it.name))
	}
	client.SendServerMessage(sb.String())
}

func printPebbleShopCategory(client *Client, cat pebbleShopCategory) {
	owned, _ := db.GetPlayerShopItems(client.Ipid())
	ownedSet := make(map[string]struct{}, len(owned))
	for _, id := range owned {
		ownedSet[id] = struct{}{}
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s %s Tags — /shop buy <item_id>\n", cat.emoji, cat.displayName))
	sb.WriteString(fmt.Sprintf("  %-22s  %-16s  %-10s  %s\n", "Item ID", "Tag", "Price", "Owned?"))
	sb.WriteString("  " + strings.Repeat("─", 68) + "\n")
	for _, it := range pebbleShopItems {
		if it.category != cat.key {
			continue
		}
		mark := ""
		if _, ok := ownedSet[it.id]; ok {
			mark = " ✅"
		}
		sb.WriteString(fmt.Sprintf("  %-22s  %-16s  %-10s  %s%s\n",
			it.id, "["+it.name+"]", fmt.Sprintf("%d Pebbles", it.price), it.description, mark))
	}
	client.SendServerMessage(sb.String())
}

func printPebbleOwned(client *Client) {
	items, err := db.GetPlayerShopItems(client.Ipid())
	if err != nil || len(items) == 0 {
		client.SendServerMessage("You don't own any Pebble tags yet. Use /shop to browse the catalog.")
		return
	}
	active := db.GetActiveTag(client.Ipid())
	var sb strings.Builder
	sb.WriteString("\n🏷️ Your Pebble tags\n")
	for _, id := range items {
		name, ok := lookupPebbleTagName(id)
		if !ok {
			// Custom or legacy tag; show it too so /mytags and /shop items
			// don't silently hide owned items.
			if n, ok := lookupTag(id); ok {
				name = n
			} else {
				continue
			}
		}
		mark := ""
		if id == active {
			mark = "  ← active"
		}
		sb.WriteString(fmt.Sprintf("  %-24s — [%s]%s\n", id, name, mark))
	}
	client.SendServerMessage(sb.String())
}

func pebbleShopBuy(client *Client, itemID string) {
	it, ok := pebbleShopItemByID(itemID)
	if !ok {
		if isGamblingLegacyTag(itemID) {
			client.SendServerMessage("That tag isn't part of the Pebble shop. Pebble Mode has no gambling-themed tags.")
			return
		}
		if legacy, ok := shopItemByID(itemID); ok && legacy.kind == shopKindTag {
			client.SendServerMessage(fmt.Sprintf("[%v] is a classic tag and is FREE in Pebble Mode. Equip it with: /settag %v", legacy.name, itemID))
			return
		}
		client.SendServerMessage(fmt.Sprintf("Unknown Pebble shop item '%v'. Use /shop to browse categories.", itemID))
		return
	}
	if db.HasShopItem(client.Ipid(), it.id) {
		client.SendServerMessage(fmt.Sprintf("You already own [%v]. Use /settag %v to equip it.", it.name, it.id))
		return
	}
	// Ensure a PEBBLES row exists so a brand-new player gets a clear
	// insufficient-balance message rather than a database error.
	if err := db.EnsurePebbleBalance(client.Ipid()); err != nil {
		client.SendServerMessage("Could not read your Pebble balance. Please try again later.")
		return
	}
	if err := db.PurchasePebbleShopItem(client.Ipid(), it.id, it.price); err != nil {
		client.SendServerMessage(fmt.Sprintf("Purchase failed: %v", err))
		return
	}
	_ = db.SetActiveTag(client.Ipid(), it.id)
	if client.Authenticated() {
		db.SetAccountActiveTag(client.ModName(), it.id) //nolint:errcheck
	}
	newBal, _ := db.GetPebbleBalance(client.Ipid())
	client.SendServerMessage(fmt.Sprintf(
		"✅ Purchased [%v] for %d Pebbles! It is now your active tag.\n"+
			"Use /settag <id> to switch tags, or /settag none to remove it.\n"+
			"Balance: %d Pebbles", it.name, it.price, newBal))
}
