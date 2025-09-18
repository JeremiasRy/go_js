function closure() {
    let count = 0;

    return {
        count: () => {
            count++;
            return count
        }
    }
}