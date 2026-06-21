import sys

def main():
    input_data = sys.stdin.read().split()
    if not input_data:
        return
    n = int(input_data[0])
    a = [int(x) for x in input_data[1:n+1]]
    
    max_so_far = 0
    curr_max = 0
    for i in range(n):
        curr_max = max(0, curr_max + a[i])
        max_so_far = max(max_so_far, curr_max)
    print(max_so_far)

if __name__ == '__main__':
    main()
