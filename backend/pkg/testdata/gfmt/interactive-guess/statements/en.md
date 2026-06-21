<!--legend -->

This is an interactive problem.
Guess the hidden number $X$ ($1 \le X \le N$) in at most 30 queries.

<!-- input -->

The first line of the input contains a single integer $N$ ($1 \le N \le 10^9$).

<!-- output -->

To make a query, output "? Y" where $1 \le Y \le N$. The judge will respond with:

- `>` if the hidden number is greater than $Y$.
- `<` if the hidden number is less than $Y$.
- `=` if the hidden number is equal to $Y$.

When you know the answer, output "! Y" and terminate.

<!-- notes -->

Make sure to flush your output stream after each query.
