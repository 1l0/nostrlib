package nostr

const (
	KindProfileMetadata          uint16 = 0
	KindTextNote                 uint16 = 1
	KindRecommendServer          uint16 = 2
	KindFollowList               uint16 = 3
	KindEncryptedDirectMessage   uint16 = 4
	KindDeletion                 uint16 = 5
	KindRepost                   uint16 = 6
	KindReaction                 uint16 = 7
	KindBadgeAward               uint16 = 8
	KindSimpleGroupChatMessage   uint16 = 9
	KindSimpleGroupThreadedReply uint16 = 10
	KindSimpleGroupThread        uint16 = 11
	KindSimpleGroupReply         uint16 = 12
	KindSeal                     uint16 = 13
	KindDirectMessage            uint16 = 14
	KindGenericRepost            uint16 = 16
	KindReactionToWebsite        uint16 = 17
	KindChannelCreation          uint16 = 40
	KindChannelMetadata          uint16 = 41
	KindChannelMessage           uint16 = 42
	KindChannelHideMessage       uint16 = 43
	KindChannelMuteUser          uint16 = 44
	KindChess                    uint16 = 64
	KindMergeRequests            uint16 = 818
	KindComment                  uint16 = 1111
	KindBid                      uint16 = 1021
	KindBidConfirmation          uint16 = 1022
	KindOpenTimestamps           uint16 = 1040
	KindGiftWrap                 uint16 = 1059
	KindFileMetadata             uint16 = 1063
	KindLiveChatMessage          uint16 = 1311
	KindPatch                    uint16 = 1617
	KindIssue                    uint16 = 1621
	KindReply                    uint16 = 1622
	KindStatusOpen               uint16 = 1630
	KindStatusApplied            uint16 = 1631
	KindStatusClosed             uint16 = 1632
	KindStatusDraft              uint16 = 1633
	KindProblemTracker           uint16 = 1971
	KindReporting                uint16 = 1984
	KindLabel                    uint16 = 1985
	KindRelayReviews             uint16 = 1986
	KindAIEmbeddings             uint16 = 1987
	KindTorrent                  uint16 = 2003
	KindTorrentComment           uint16 = 2004
	KindCoinjoinPool             uint16 = 2022
	KindCommunityPostApproval    uint16 = 4550
	KindJobFeedback              uint16 = 7000
	KindSimpleGroupPutUser       uint16 = 9000
	KindSimpleGroupRemoveUser    uint16 = 9001
	KindSimpleGroupEditMetadata  uint16 = 9002
	KindSimpleGroupDeleteEvent   uint16 = 9005
	KindSimpleGroupCreateGroup   uint16 = 9007
	KindSimpleGroupDeleteGroup   uint16 = 9008
	KindSimpleGroupCreateInvite  uint16 = 9009
	KindSimpleGroupJoinRequest   uint16 = 9021
	KindSimpleGroupLeaveRequest  uint16 = 9022
	KindZapGoal                  uint16 = 9041
	KindNutZap                   uint16 = 9321
	KindTidalLogin               uint16 = 9467
	KindZapRequest               uint16 = 9734
	KindZap                      uint16 = 9735
	KindHighlights               uint16 = 9802
	KindMuteList                 uint16 = 10000
	KindPinList                  uint16 = 10001
	KindRelayListMetadata        uint16 = 10002
	KindBookmarkList             uint16 = 10003
	KindCommunityList            uint16 = 10004
	KindPublicChatList           uint16 = 10005
	KindBlockedRelayList         uint16 = 10006
	KindSearchRelayList          uint16 = 10007
	KindSimpleGroupList          uint16 = 10009
	KindInterestList             uint16 = 10015
	KindNutZapInfo               uint16 = 10019
	KindEmojiList                uint16 = 10030
	KindDMRelayList              uint16 = 10050
	KindUserServerList           uint16 = 10063
	KindFileStorageServerList    uint16 = 10096
	KindGoodWikiAuthorList       uint16 = 10101
	KindGoodWikiRelayList        uint16 = 10102
	KindNWCWalletInfo            uint16 = 13194
	KindLightningPubRPC          uint16 = 21000
	KindClientAuthentication     uint16 = 22242
	KindNWCWalletRequest         uint16 = 23194
	KindNWCWalletResponse        uint16 = 23195
	KindNostrConnect             uint16 = 24133
	KindBlobs                    uint16 = 24242
	KindHTTPAuth                 uint16 = 27235
	KindCategorizedPeopleList    uint16 = 30000
	KindCategorizedBookmarksList uint16 = 30001
	KindRelaySets                uint16 = 30002
	KindBookmarkSets             uint16 = 30003
	KindCuratedSets              uint16 = 30004
	KindCuratedVideoSets         uint16 = 30005
	KindMuteSets                 uint16 = 30007
	KindProfileBadges            uint16 = 30008
	KindBadgeDefinition          uint16 = 30009
	KindInterestSets             uint16 = 30015
	KindStallDefinition          uint16 = 30017
	KindProductDefinition        uint16 = 30018
	KindMarketplaceUI            uint16 = 30019
	KindProductSoldAsAuction     uint16 = 30020
	KindArticle                  uint16 = 30023
	KindDraftArticle             uint16 = 30024
	KindEmojiSets                uint16 = 30030
	KindModularArticleHeader     uint16 = 30040
	KindModularArticleContent    uint16 = 30041
	KindReleaseArtifactSets      uint16 = 30063
	KindApplicationSpecificData  uint16 = 30078
	KindLiveEvent                uint16 = 30311
	KindUserStatuses             uint16 = 30315
	KindClassifiedListing        uint16 = 30402
	KindDraftClassifiedListing   uint16 = 30403
	KindRepositoryAnnouncement   uint16 = 30617
	KindRepositoryState          uint16 = 30618
	KindSimpleGroupMetadata      uint16 = 39000
	KindSimpleGroupAdmins        uint16 = 39001
	KindSimpleGroupMembers       uint16 = 39002
	KindSimpleGroupRoles         uint16 = 39003
	KindWikiArticle              uint16 = 30818
	KindRedirects                uint16 = 30819
	KindFeed                     uint16 = 31890
	KindDateCalendarEvent        uint16 = 31922
	KindTimeCalendarEvent        uint16 = 31923
	KindCalendar                 uint16 = 31924
	KindCalendarEventRSVP        uint16 = 31925
	KindHandlerRecommendation    uint16 = 31989
	KindHandlerInformation       uint16 = 31990
	KindVideoEvent               uint16 = 34235
	KindShortVideoEvent          uint16 = 34236
	KindVideoViewEvent           uint16 = 34237
	KindCommunityDefinition      uint16 = 34550
)

func IsRegularKind(kind uint16) bool {
	return kind < 10000 && kind != 0 && kind != 3
}

func IsReplaceableKind(kind uint16) bool {
	return kind == 0 || kind == 3 || (10000 <= kind && kind < 20000)
}

func IsEphemeralKind(kind uint16) bool {
	return 20000 <= kind && kind < 30000
}

func IsAddressableKind(kind uint16) bool {
	return 30000 <= kind && kind < 40000
}
