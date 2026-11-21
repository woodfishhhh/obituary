// Every day is one column of the same width, the "later" column too. A column
// never shrinks below what a card needs to stay readable; a window too narrow
// for all of them scrolls the board sideways (today is the first column).
export const MIN_COLUMN = 110; // px; eight days and "later" fit the 1040 px window the app opens at

export const columnTemplate = (cols: number) => `repeat(${cols}, minmax(${MIN_COLUMN}px, 1fr))`;
