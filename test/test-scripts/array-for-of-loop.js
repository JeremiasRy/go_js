const arr = [1, hello(), 3, 4, 5, 6, 7]
function hello() {
    return 2
}
for (const item of arr) {
    console.log(item)
}

function locallyDefinedIterator() {
    for (const item of arr) {
        console.log(item)
    }
}