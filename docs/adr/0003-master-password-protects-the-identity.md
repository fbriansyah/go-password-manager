# The Master Password protects the Identity, not the Secrets

The Master Password never encrypts a Secret directly. It only opens the Identity file (a private key encrypted with a passphrase). Reading a Secret therefore needs two things: the Identity file and the Master Password. Writing one needs only the Recipient — a machine can add Secrets without ever being able to read them. That write-without-read property is the single reason this design is asymmetric; drop it and plain symmetric encryption would be simpler.

There is no recovery mechanism: losing the Identity or forgetting the Master Password means every Secret is gone permanently. `gopm init` must say so.
