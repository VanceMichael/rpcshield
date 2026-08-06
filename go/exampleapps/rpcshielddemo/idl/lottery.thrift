struct PlayRequest {
    1: required i32 lucky
}

struct PlayResponse {
    1: required bool won
}

// Lottery plays the lottery, checking whether the chosen number is a winner.
service Lottery {
    PlayResponse play(1: PlayRequest req)
}
