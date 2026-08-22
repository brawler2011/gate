#include <iostream>
#include <string>

using namespace std;

int main() {
    int n;
    if (!(cin >> n)) return 0;
    
    int low = 1, high = n;
    while (low <= high) {
        int mid = low + (high - low) / 2;
        cout << "? " << mid << endl;
        
        string response;
        if (!(cin >> response)) break;
        
        if (response == "=") {
            cout << "! " << mid << endl;
            break;
        } else if (response == ">") {
            low = mid + 1;
        } else {
            high = mid - 1;
        }
    }
    return 0;
}
