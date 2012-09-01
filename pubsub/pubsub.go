package pubsub

import (

)

// TODO: A channel-based pub-sub framework
// First use case is broadcasting signal.Notify channel to any number of
// receivers (i.e. many parts of program want SIGINT notification, maybe also
// SIGKILL notification)