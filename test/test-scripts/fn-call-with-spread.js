function callMe(one, two, three, four) {
    return `${one} ${two} ${three} ${four}`
}
const arr = ["hi", "my", "mane", "is"]
const name = "Earl"

let i = 0
while (i < 5) {
    const result = callMe(...arr)
    if (i < 4) {
        i++
        continue
    }
    console.log(result, name)
    i++
}
