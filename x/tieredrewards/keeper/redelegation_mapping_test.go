package keeper_test

func (s *KeeperSuite) TestDeletePositionRedelegationMappings_RemovesTargetPosition() {
	targetPos := uint64(42)
	otherPos := uint64(43)

	targetUnb1 := uint64(100)
	targetUnb2 := uint64(101)
	otherUnb := uint64(200)

	s.Require().NoError(s.keeper.RedelegationMappings.Set(s.ctx, targetUnb1, targetPos))
	s.Require().NoError(s.keeper.RedelegationMappings.Set(s.ctx, targetUnb2, targetPos))
	s.Require().NoError(s.keeper.RedelegationMappings.Set(s.ctx, otherUnb, otherPos))

	s.Require().NoError(s.keeper.DeletePositionRedelegationMappings(s.ctx, targetPos))

	// Both target rows removed.
	for _, id := range []uint64{targetUnb1, targetUnb2} {
		has, err := s.keeper.RedelegationMappings.Has(s.ctx, id)
		s.Require().NoError(err)
		s.Require().False(has, "mapping for target position should be deleted: unbonding_id=%d", id)
	}

	// Unrelated row untouched.
	has, err := s.keeper.RedelegationMappings.Has(s.ctx, otherUnb)
	s.Require().NoError(err)
	s.Require().True(has, "mapping for other position must survive")

	// Idempotent: calling again on a position with no mappings is a no-op.
	s.Require().NoError(s.keeper.DeletePositionRedelegationMappings(s.ctx, targetPos))
}
