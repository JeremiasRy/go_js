const str = "x-0\nx-1\nx-2"

for (const s of str.split("\n")) {
    const [ch, num] = s.split("-")
    console.log(ch, num)
}