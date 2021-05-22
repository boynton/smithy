namespace smithy.example

@documentation("""
    <div>
        <p>Hello!</p>
    </div>
    """)
structure MyStruct {
    @documentation("""
        xxx
           yyy
        zzz
	""")
	foo: String
}

///<foo>
string MyString

@documentation("The seven-day average dipped to 27,815 on Friday, the lowest since June 22 and less than a tenth of the infection rate during the winter surge, according to state health department data compiled by The Washington Post.")
string Blah
