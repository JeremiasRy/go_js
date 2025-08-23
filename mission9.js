const a = 1

function hello() {
    const b = 1
    {
        const c = 2
        console.log(b)
        {
            const d = 3
            console.log(d)
        }
        {
            const e = 4
            console.log(d)
            const a = console.log(c)
            console.log(b)
        }
    }
}
