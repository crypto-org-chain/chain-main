> Copyright (c) 2016-2021 Shanghai Bianjie AI Technology Inc. (licensed under the Apache License, Version 2.0)
> Modifications Copyright (c) 2021-present Crypto.org (licensed under the Apache License, Version 2.0)

# Future Improvements

There's interesting work that could be done about moving tokenData into its own module. This could act as one of the `tokenURI` endpoints if a chain chooses to offer storage as a solution. Furthermore on-chain tokenData can be trusted to a higher degree and might be used in secondary actions like price evaluation. Moving tokenData to it's own module could be useful for the Bank Module as well. It would be able to describe attributes like decimal places and information regarding vesting schedules. It would be needed to have a level of introspection to describe the content without actually delivering the content for client libraries to interact with it. Using schema.org as a common location to settle tokenData schema structure would be a good and impartial place to do so.

Inter-Blockchain Communication will need to develop its own Message types that allow NFTs to be transferred across chains. Making sure that spec is able to support the NFTs created by this module should be easy. What might be more complicated is a transfer that includes optional tokenData so that a receiving chain has the option of parsing and storing it instead of making IBC queries when that data needs to be accessed (assuming that information stays up to date).
