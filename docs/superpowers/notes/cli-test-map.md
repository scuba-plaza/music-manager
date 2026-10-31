# Where each CLI test's behaviour lives now

Task 3 deletes `internal/cli`. This map is what it consults first: one row for each of the 56 tests in `internal/cli/*_test.go`, made with `grep -hn "^func Test" internal/cli/*_test.go`.

Verdicts:

- **dropped**: only command-line parsing, flags, help, output layout or a feature the spec removes (D1: `review apply`, `playlist import`, the interactive `mm add` form). Nothing to keep.
- **covered**: the behaviour is pinned by an existing test that was read, and for the behaviours of the checklist below also broken on purpose to see it fail.
- **NEW**: the existing tests pinned only part of what the CLI test pinned (a message, a refusal that changes nothing, the web side of it); a test was written in the commit that added this file. Each was watched failing with the one line it guards switched off.

Test names are given without `Test` in the shortened form `app/...` (`internal/app`), `web/...` (`internal/web`), `browse/...`, `manifest/...`, `device/...`.

## The map

| CLI test | Verdict | Where it lives |
|---|---|---|
| TestAddWithFlagsIsNonInteractive | covered | the flags are dropped; the details of an added track: web/TestATrackIsFetchedFromALinkAndAdded, app/TestFinishSavesTheTrackWithTheGivenDetails (source link, cuts, ids), app/TestEditTitleAndArtistIdentifiesTheTrack |
| TestAddWithoutFlagsAsksOnlyWhenAPrompterIsAvailable | dropped | the interactive form goes with `Prompter` (D1); the page's form is pre-filled from the download: app/TestFetchOffersAConfidentMatchAndKeepsWhatTheDownloadSaid, web/TestATrackIsFetchedFromALinkAndAdded |
| TestAddReportsProblemsClearly | covered | no artist or bad cut: app/TestFinishFailuresAddNothing, web/TestATrackIsFetchedFromALinkAndAdded; duplicate: app/TestFetchRefusesADuplicate, web/TestAnAddedTrackThatIsThereAlreadyIsSaidSo; no downloader or link: web/TestAddingNeedsADownloaderAndALink; the cover given as a file path is a command-line argument and is dropped |
| TestCreatingListingAndShowingPlaylists | covered | app/TestAPlaylistKeepsItsTracksInOrderAcrossReloads, app/TestAddingAnUnknownPlaylistOrTrackFailsAndChangesNothing, app/TestCreatingAPlaylistWithTracksIsAllOrNothing, web/TestGroupsBulkAndPlaylistsOverHTTP |
| TestAddingRemovingAndMovingTracksOfAPlaylist | covered | app/TestATrackIsInAPlaylistOnlyOnce, app/TestMovingATrackWithinAPlaylist (out-of-range places refused), app/TestRemovingTracksFromAPlaylist (a track that is not in it fails the whole request) |
| TestRenamingAndDeletingAPlaylist | covered | app/TestRenamingAPlaylistKeepsItsTracksAndPlace, app/TestPlaylistNamesMustBeUsableAndDistinct, app/TestDeletingAPlaylistLeavesTheTracks |
| TestSyncWritesThePlaylistsMadeOnTheCommandLine | covered | app/TestSyncWritesAnM3U8AnyPlayerCanRead |
| TestImportingAPlaylistFileOfThePlayer | dropped | `playlist import` is removed by D1; `internal/device/wolfgangpl.go` and its test go in Task 3 |
| TestAPlaylistHasALatinNameForTheTargetsThatCannotShowTheTrueOne | covered | app/TestAPlaylistsLatinNameIsSetByHandAndOnlyTheWolfgangFileFollowsIt (Wolfgang file by the Latin name, library folder by the true one, clearing forgets it), device/TestAPlaylistFileIsNamedByTheTrueNameOrTheLatinOne (the worked-out name), browse/TestAPlaylistIsRenamedAndGivenALatinName |
| TestIngestThenList | covered | app/TestIngestSkipsAudioAlreadyInTheLibrary (one added, one duplicate; a second ingest adds nothing), app/TestIngestKeepsOldTagsAndFilenameAsHintsOnly, web/TestMP3FilesAreAddedFromUploads; the `ls` columns are dropped |
| TestEditThenShow | covered | app/TestEditTitleAndArtistIdentifiesTheTrack, app/TestEditTrims, app/TestSearchFindsATrackByTheLatinFormOfItsTrueName ("firuz" finds the Arabic artist), web/TestAFieldIsEditedOverHTTPAndUndone; the `show` layout is dropped |
| TestEditRejectsBadInput | covered | app/TestEditTrims (trims that leave nothing), app/TestEditUnknownID, browse/TestWhatWouldBreakATrackIsRefused (bad times name the field), web/TestWhatCannotBeDoneIsA422WithTheReason; "nothing to change" is a command-line check, the page says it for bulk edits (web/TestGroupsBulkAndPlaylistsOverHTTP) |
| TestSyncDryRunThenApply | covered | app/TestPlanSyncWritesNothing, app/TestSyncRendersTheTreeFromTheManifest (the plan, applied, creates the Arabic path; a second sync has nothing to do), web/TestASyncToAnotherFolderIsPlannedThenWritten, web/TestSyncingTheLibraryFolderRecordsItAndClearsUndo |
| TestSyncPruneNeedsConfirmation | covered | web/TestStraysAreOnlyDeletedWhenAskedAndConfirmed (strays only reported, deleting needs the word, empty folder removed), app/TestSyncPruneRemovesEmptyDirsButLeavesOtherFilesAndInternals |
| TestSyncToATarget | covered | app/TestSyncToAnotherTargetLeavesTheLibraryRootAlone, web/TestASyncToAnotherFolderIsPlannedThenWritten |
| TestSyncWolfgangNeedsATarget | covered | web/TestSyncingTheLibraryFolderRecordsItAndClearsUndo (a plan for the library folder with `wolfgang` set still writes the standard layout); refusing the flag combination is dropped |
| TestSyncWolfgangSaysWhatItWritesFor | NEW | web/TestAPlanForTheWolfgangNamesItAndWarnsAboutWhatItWillNotShowWell (a Wolfgang plan names the player, a plain one does not); the files written for it: app/TestSyncWritesTagsTheWholePlayerCanShow, app/TestSyncWritesAPlaylistTheWolfgangPlayerCanRead |
| TestEditCover | covered | app/TestChangingAndRemovingACover (replaced cover is rewritten, a cleared one is pruned), app/TestIngestKeepsEmbeddedArtworkAndSyncWritesIt, web/TestCoversAreUploadedCheckedAndRemoved; a missing cover file is a command-line argument and is dropped |
| TestRemoveThenSyncCollects | covered + NEW | app/TestRemoveDeletesTheRecordAndSyncCollectsItsBlobs (blobs stay until a sync, which collects them; the rendered file stays until prune), app/TestRemoveManyRemovesAllOrNone; NEW app/TestRemovingAnUnknownIDSaysWhichOneAndRemovesNothing |
| TestDoctor | covered + NEW | app/TestDoctorOnAHealthyLibrary, app/TestDoctorFindsDamagedStores, app/TestDoctorWarnsAboutUnidentifiedTracksAndClashingPositions, web/TestTheLibraryCheckSaysWhatIsWrong; NEW web/TestTheLibraryCheckReportsAMissingFileAsAnError, web/TestTheLibraryCheckOfALibraryWithNothingWrongFindsNothing |
| TestVersionFlag | dropped | a command-line flag; Task 2 tests `--version` in `internal/launch` |
| TestEditTheReleaseDateAndShowAllThreeDates | covered + NEW | app/TestEditReleaseDate, app/TestEditRejectsDatesThatAreNotDates (the message shows the format, the record is unchanged), app/TestEditingTheReleaseDateCountsAsAnEdit; NEW web/TestTheInspectorSaysWhenATrackWasReleasedAddedAndEdited |
| TestAddTakesTheReleaseDateFromAFlag | NEW | web/TestATrackIsAddedWithItsReleaseDate, web/TestATrackIsNotAddedWithAReleaseDateThatIsNotADate |
| TestListSortedByAnyField | covered + NEW | app/TestSortingTracksByEachField (all eight keys, both ways, missing values last), app/TestSortKeysHaveNamesAndANaturalDirection, app/TestSortValueIsWhatTheListShowsBesideATrack; NEW web/TestTheLibraryCanBeSortedByEveryKeyBothWaysAndStillFiltered, web/TestSortingByADateShowsTheDateBesideEachTrack, web/TestASortKeyThatDoesNotExistSaysWhichDo |
| TestEditTheSourceLink | covered + NEW | app/TestEditSourceURL, app/TestEditRejectsSourcesThatAreNotLinks (the message shows what a link looks like, the record is unchanged); NEW web/TestTheInspectorSaysWhereATrackCameFrom; "add has no source flag" is dropped |
| TestEditTheGenreAndFindTracksByIt | covered + NEW | app/TestEditGenre, app/TestOtherEditsLeaveTheGenreAlone, manifest/TestSearchFindsAGenreAsAWordOrWithTheGenrePrefix; NEW web/TestAGenreIsSetOverHTTPAndFoundByTheGenreFilter |
| TestAddTakesTheGenreFromAFlag | NEW | web/TestATrackIsAddedWithItsGenre |
| TestShowSaysWhatTheWolfgangWillShowOnlyWhenAsked | covered + NEW | browse/TestATrackHasThreeTabsAndAFourthUnderWolfgang, web/TestATracksInspectorAndItsCover (three tabs unless asked); NEW web/TestTheWolfgangTabSaysWhatThePlayerWillShowAndWhatItWillNotShowWell; the removed `--player-*` flags are dropped |
| TestSetTheVolumeAndTheFadesFromTheCommandLine | covered | app/TestGainInDecibelsIsKeptAsWholeSteps, app/TestEditFades, app/TestAFadeIsTypedAsALengthOrAsAStretch (counted from the cuts), app/TestAFadeIsShownTheWayItCanBeTypedAgain, app/TestAVolumeIsWrittenAsDecibelsAndSteps, browse/TestVolumeAndFadesAreNudgedAndCleared, web/TestNudgesAndClearsOverHTTP |
| TestVolumeAndFadeFlagsSayWhatIsWrong | NEW | app/TestAVolumeThatIsNotANumberIsRefusedWithAMessageThatNamesIt, app/TestAVolumeBeyondTheLimitIsRefusedWithAMessageThatSaysTheLimit, app/TestAFadeThatIsNotATimeIsRefusedWithAMessageThatNamesIt, app/TestAFadeLongerThanTheTrackIsRefusedWithAMessageThatNamesTheFade, app/TestFadesThatCrossAreRefusedWithAMessageThatSaysWhy |
| TestSyncWolfgangListsWhatThePlayerWillNotShowWell | covered + NEW | app/TestPlanSyncCarriesTheWarningsOfItsDevice (the Wolfgang warns, the generic target does not), browse/TestAPlanNamesTheTracksItWarnsAbout; NEW web/TestAPlanForTheWolfgangNamesItAndWarnsAboutWhatItWillNotShowWell |
| TestShowWolfgangSaysWhatTheWolfgangWillNotShowWell | NEW | web/TestTheWolfgangTabSaysWhatThePlayerWillShowAndWhatItWillNotShowWell |
| TestEditSetsTheLatinFormsAndShowSaysSoOnlyWhereTheyAreSet | covered + NEW | app/TestEditSetsAndClearsTheLatinForms, app/TestALatinFormGoesWhenItsNameDoes; NEW web/TestTheLatinTabShowsAFormSetByHandAsSetAndAWorkedOutFormDimly |
| TestShowWolfgangUsesTheLatinFormsSetByHand | covered | app/TestSyncOutputGolden (`sync-wolfgang.golden` has artist `A. H. Hafez` and album `Rawa'e` from forms set by hand), device/TestTheWolfgangWritesTheTagsFromTheLatinFormsSetByHand |
| TestEditRefusesALatinFormOnSeveralLines | NEW | app/TestALatinFormOnSeveralLinesIsRefusedWithAMessageAndChangesNothing (app/TestALatinFormIsOneLine pins only the refusal) |
| TestRemoveWithARepeatedIDRemovesNothing | NEW | app/TestRemovingWithARepeatedIDRemovesNothing (app/TestRemoveManyRemovesAllOrNone checks only that an error comes, not its message, and sees the track count only indirectly) |
| TestEditWithACoverThatIsNotAnImageChangesNothing | NEW | app/TestACoverThatIsNotAnImageChangesNothing, web/TestAPictureThatIsNotACoverLeavesTheCoverAsItWas |
| TestOnlyTheRightSpellingOfWolfgangIsAFlag | dropped | flag parsing; Task 2 tests that an unknown flag fails and names itself |
| TestThereIsNoCommandToConvertCovers | dropped | the absence of a command; covers are kept as given: app/TestACoverIsStoredExactlyAsGiven |
| TestApplyingAReviewFromTheCommandLine | dropped | `review apply` is removed by D1; `ParseReview`, `ApplyReview` and `internal/app/review_test.go` go in Task 3 |
| TestAMistakeInAReviewFileIsReportedAndNothingIsChanged | dropped | same: the review file format is removed |
| TestIdentifyYes | covered | app/TestIdentifyTakesAConfidentTopMatch, web/TestATrackIsLookedUpAndIdentifiedOverHTTP, web/TestEveryTrackThatNeedsWorkIsIdentifiedAsAJob |
| TestIdentifyInteractiveChoice | dropped | the terminal prompt is removed with `Prompter`; picking one of the candidates: app/TestApplyCandidateIdentifiesTheTrackAsChosen, web/TestATrackIsLookedUpAndIdentifiedOverHTTP |
| TestIdentifyReportsSkipsAndFailures | covered + NEW | app/TestIdentifySkipsWeakMatchesAndMisses, app/TestIdentifyOneFailureDoesNotStopTheRest; NEW web/TestIdentifyingEverythingLeavesTracksAloneWhoseBestMatchIsWeak, web/TestAFailedLookupIsReportedAndTheOtherTracksAreStillIdentified (what the page is told); the `scores only 60` wording is shown by nothing but the CLI and is dropped |
| TestIdentifyNeedsASourceAndAnswers | covered | web/TestWithoutADatabaseThereIsNothingToLookUp, app/TestIdentifyLeavesIdentifiedTracksAloneUnlessNamed (an unknown id is refused); "no terminal and no --yes" goes with the prompter |
| TestAddLooksTheTrackUpAndUsesItAsTheDefault | covered | app/TestAddUsesAConfidentMatchAsDefaultsAndRecordsItsIDs, app/TestAddWarnsButCarriesOnWhenTheLookupFails, app/TestFetchOffersAConfidentMatchAndKeepsWhatTheDownloadSaid; `--no-lookup` is a flag and is dropped |
| TestAdopt | covered | app/TestAdoptTakesWhatTheOldTagsSaid, app/TestAdoptLeavesTracksThatLackATitleOrArtist, app/TestAdoptOnlyTouchesUnidentifiedTracksAndNamedOnes (an unknown id is refused), web/TestOldTagsAreAdoptedOverHTTP |
| TestSyncWritesReplayGainWhenTheProgramHasAMeter | covered + NEW | app/TestSyncWritesReplayGainTagsFromTheMeasuredLoudness; NEW app/TestSyncWritesNoReplayGainTagsWhenThereIsNoMeter; handing the meter in through `Deps` is dropped (Task 2 calls `UseLoudness` in main) |
| TestWebServesTheLibraryOnTheDefaultAddress | dropped | ported to `internal/launch` in Task 2 (`TestNoArgumentsStartsTheServerOnTheDefaultAddress`) |
| TestWebTakesItsAddressAndFlags | dropped | ported to Task 2 (`TestAddrAndNoOpenFlagsArePassedOn`); `--wolfgang` goes in Task 22 |
| TestWebFailsWithoutAServer | dropped | a wiring check of the command tree |
| TestWebPassesOnWhyItStopped | dropped | ported to Task 2 (`TestAServerErrorIsReported`) |
| TestTheDefaultLibraryIsMusicLibrary | dropped | ported to Task 2 (`TestTheDefaultLibraryIsMusicLibrary` in `internal/launch`) |
| TestPlainMMPrintsHelpThatPointsToTheWebInterface | dropped | help text |
| TestAnUnknownCommandStillFails | dropped | there are no commands any more |
| TestTheWolfgangPreviewIsAFlagOfWebOnly | dropped | flag parsing |

## The behaviours Task 1 had to check one by one

| Behaviour | Pinned by |
|---|---|
| sync dry-run then apply | app/TestPlanSyncWritesNothing, app/TestSyncRendersTheTreeFromTheManifest, web/TestASyncToAnotherFolderIsPlannedThenWritten |
| prune only with confirmation | web/TestStraysAreOnlyDeletedWhenAskedAndConfirmed |
| remove then sync collects blobs | app/TestRemoveDeletesTheRecordAndSyncCollectsItsBlobs |
| removing with a repeated ID removes nothing | NEW app/TestRemovingWithARepeatedIDRemovesNothing |
| a cover that is not an image changes nothing | NEW app/TestACoverThatIsNotAnImageChangesNothing, NEW web/TestAPictureThatIsNotACoverLeavesTheCoverAsItWas |
| volume and fade values are validated with a message that says what is wrong | NEW five tests in `internal/app/loudness_test.go` (rows above) |
| a Latin form on several lines is refused | NEW app/TestALatinFormOnSeveralLinesIsRefusedWithAMessageAndChangesNothing |
| ReplayGain tags written with a meter, none without | app/TestSyncWritesReplayGainTagsFromTheMeasuredLoudness, NEW app/TestSyncWritesNoReplayGainTagsWhenThereIsNoMeter |
| a playlist's Latin name resolves per target | app/TestAPlaylistsLatinNameIsSetByHandAndOnlyTheWolfgangFileFollowsIt, device/TestAPlaylistFileIsNamedByTheTrueNameOrTheLatinOne |
| sorting by every field | app/TestSortingTracksByEachField, NEW web/TestTheLibraryCanBeSortedByEveryKeyBothWaysAndStillFiltered |
| release date plus the three dates | app/TestEditReleaseDate, NEW web/TestTheInspectorSaysWhenATrackWasReleasedAddedAndEdited, NEW web/TestATrackIsAddedWithItsReleaseDate |
| genre filter | manifest/TestSearchFindsAGenreAsAWordOrWithTheGenrePrefix, NEW web/TestAGenreIsSetOverHTTPAndFoundByTheGenreFilter |
| source link edit | app/TestEditSourceURL, app/TestEditRejectsSourcesThatAreNotLinks, NEW web/TestTheInspectorSaysWhereATrackCameFrom |
| doctor | app/TestDoctorFindsDamagedStores, NEW web/TestTheLibraryCheckReportsAMissingFileAsAnError |
| ingest is idempotent | app/TestIngestSkipsAudioAlreadyInTheLibrary, web/TestMP3FilesAreAddedFromUploads |

## For Task 3

- Tests that go with deleted code: `internal/app/review_test.go` and `internal/device/wolfgangpl_test.go`.
- Tests that script a `Prompter` and lose the prompt with it: `TestAddAsksEachFieldWithDefaultsFromTheDownload`, `TestAddOnlyAsksForFieldsNotGivenAsFlags`, `TestAddNeverAsksForALatinForm`, `TestAddAnswerValidation`, `TestAddValidatesTheReleaseDateAnswer`, `TestIdentifyAsksWhichCandidateAndLetsYouSkip`, `TestIdentifyWithoutAnswersAvailableIsAnError`, and the prompted halves of `TestAddAsksForAGenreUnlessOneIsGiven`, `TestAddOffersTheReleaseDateOfTheMatchAndTakesWhatIsTyped`, `TestAddKeepsTheArtistAsTheMatchWritesIt`, `TestAddIgnoresWeakMatchesAndMatchesTheUserOverrode` and `TestAddFailuresLeaveTheLibraryUntouched` (`user aborts`). Their non-prompted halves stay.
- None of the NEW tests uses a `Prompter`. Two of them are about the legacy Wolfgang shape that Tasks 7 and 22 replace and change with it: `web/TestAPlanForTheWolfgangNamesItAndWarnsAboutWhatItWillNotShowWell` (the `wolfgang` request field) and `web/TestTheWolfgangTabSaysWhatThePlayerWillShowAndWhatItWillNotShowWell` (the `wolfgang` tab payload).
