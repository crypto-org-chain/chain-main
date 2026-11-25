{
  // Hermes cannot pick a compat mode from the CometBFT version these chains report.
  // Like pystarport, this treats every top-level key except `relayer` as a chain.
  chains(config): [
    { id: id, compat_mode: '0.37' }
    for id in std.objectFields(config)
    if id != 'relayer'
  ],
}
