package ui

import (
	"log"
	"sort"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/rivo/tview"
)

type ChannelsView struct {
	*tview.TreeView
	selectedChannel *discord.Channel
	app             *Application
}

func newChannelsView(app *Application) *ChannelsView {
	v := &ChannelsView{
		TreeView: tview.NewTreeView(),
		app:      app,
	}

	v.SetRoot(tview.NewTreeNode(""))
	v.SetTopLevel(1)
	v.SetSelectedFunc(v.onSelected)

	v.SetTitle("Channels")
	v.SetTitleAlign(tview.AlignLeft)
	v.SetBorder(true)
	v.SetBorderPadding(0, 0, 1, 1)

	return v
}

func (v *ChannelsView) onSelected(node *tview.TreeNode) {
	v.selectedChannel = nil
	v.app.view.MessagesView.selectedMessage = -1
	v.app.view.MessagesView.
		Highlight().
		Clear().
		SetTitle("")
	v.app.view.InputView.SetText("")

	ref := node.GetReference()
	c, err := v.app.state.Cabinet.Channel(ref.(discord.ChannelID))
	if err != nil {
		return
	}

	// If the channel is a category channel, expand the selected node if it is collapsed, otherwise collapse.
	if c.Type == discord.GuildCategory {
		node.SetExpanded(!node.IsExpanded())
		return
	}

	v.selectedChannel = c
	v.app.SetFocus(v.app.view.InputView)

	title := channelToString(*c)
	if c.Topic != "" {
		title += " - " + parseMarkdown(c.Topic)
	}
	v.app.view.MessagesView.SetTitle(title)

	go func() {
		// The returned slice will be sorted from latest to oldest.
		ms, err := v.app.state.Messages(c.ID, v.app.config.MessagesLimit)
		if err != nil {
			log.Println(err)
			return
		}

		for i := len(ms) - 1; i >= 0; i-- {
			_, err = v.app.view.MessagesView.Write(buildMessage(v.app, ms[i]))
			if err != nil {
				log.Println(err)
				continue
			}
		}

		v.app.view.MessagesView.ScrollToEnd()
	}()
}

func (v *ChannelsView) createChannelNode(c discord.Channel) *tview.TreeNode {
	channelNode := tview.NewTreeNode(channelToString(c))
	channelNode.SetReference(c.ID)

	return channelNode
}

func (v *ChannelsView) createPrivateChannelNodes(root *tview.TreeNode) {
	cs, err := v.app.state.Cabinet.PrivateChannels()
	if err != nil {
		log.Println(err)
		return
	}

	sort.Slice(cs, func(i, j int) bool {
		return cs[i].LastMessageID > cs[j].LastMessageID
	})

	for _, c := range cs {
		root.AddChild(v.createChannelNode(c))
	}
}

func (v *ChannelsView) createGuildChannelNodes(root *tview.TreeNode, gID discord.GuildID) {
	cs, err := v.app.state.Cabinet.Channels(gID)
	if err != nil {
		log.Println(err)
		return
	}

	sort.Slice(cs, func(i, j int) bool {
		return cs[i].Position < cs[j].Position
	})

	v.createOrphanChannelNodes(root, cs)
	v.createCategoryChannelNodes(root, cs)
	v.createChildrenChannelNodes(root, cs)
}

func (v *ChannelsView) createOrphanChannelNodes(root *tview.TreeNode, cs []discord.Channel) {
	for _, c := range cs {
		if (c.Type == discord.GuildText || c.Type == discord.GuildNews) && (!c.ParentID.IsValid()) {
			root.AddChild(v.createChannelNode(c))
		}
	}
}

func (v *ChannelsView) createCategoryChannelNodes(root *tview.TreeNode, cs []discord.Channel) {
CATEGORY:
	for _, c := range cs {
		if c.Type == discord.GuildCategory {
			for _, nestedChannel := range cs {
				if nestedChannel.ParentID == c.ID {
					root.AddChild(v.createChannelNode(c))
					continue CATEGORY
				}
			}

			root.AddChild(v.createChannelNode(c))
		}
	}
}

func (v *ChannelsView) createChildrenChannelNodes(root *tview.TreeNode, cs []discord.Channel) {
	for _, c := range cs {
		if (c.Type == discord.GuildText || c.Type == discord.GuildNews) && (c.ParentID.IsValid()) {
			var parentNode *tview.TreeNode
			root.Walk(func(node, _ *tview.TreeNode) bool {
				if node.GetReference() == c.ParentID {
					parentNode = node
					return false
				}

				return true
			})

			if parentNode != nil {
				parentNode.AddChild(v.createChannelNode(c))
			}
		}
	}
}
