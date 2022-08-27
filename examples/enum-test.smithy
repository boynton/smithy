namespace examples

///this is deprecated in v2
@enum([
		 { "name": "DIAMOND", "value": "diamond"},
		 { "name": "CLUB", "value": "club"},
		 { "name": "HEART", "value": "heart"},
		 { "name": "SPADE", "value": "spade"},
])
string Suit1

///this is deprecated in v2
@enum([
		 { "name": "DIAMOND", "value": 1},
		 { "name": "CLUB", "value": 2},
		 { "name": "HEART", "value": 3},
		 { "name": "SPADE", "value": 4},
])
integer Suit11

///values default to the name
enum Suit2a {
    DIAMOND
    CLUB
    HEART
    SPADE
}

intEnum Suit22a {
    DIAMOND = 1
    CLUB = 2
    HEART = 3
    SPADE = 4
}

///explicit values
enum Suit2b {
    DIAMOND = "diamond"
    CLUB = "club"
    HEART = "heart"
    SPADE = "spade"
}

///this is equivalent to Suit2b
enum Suit2c {
    @enumValue("diamond")
    DIAMOND
    @enumValue("club")
    CLUB
    @enumValue("heart")
    HEART
    @enumValue("spade")
    SPADE
}
