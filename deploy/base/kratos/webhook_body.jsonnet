function(ctx) {
  userId: ctx.identity.id,
  username: ctx.identity.traits.username,
  email: ctx.identity.traits.email,
}
