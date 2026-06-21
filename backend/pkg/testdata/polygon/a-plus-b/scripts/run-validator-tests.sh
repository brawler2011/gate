#!/usr/bin/env bash
echo "Running 2 validator test(s)"
echo "Running 2 validator test(s)" 1> validator-tests.log
echo "Running test #1" 1>> validator-tests.log
echo "Validator comment:" 1>> validator-tests.log
wine files/v.exe < files/tests/validator-tests/01 2>> validator-tests.log
if [ "$?" -eq "0" ]; then
    echo "Validator returned zero exit code for a invalid test" 1>> validator-tests.log
    echo "Validator returned zero exit code for a invalid test. See validator-tests.log for validator comment"
    read
fi
echo "Running test #2" 1>> validator-tests.log
echo "Validator comment:" 1>> validator-tests.log
wine files/v.exe < files/tests/validator-tests/02 2>> validator-tests.log
if [ "$?" -eq "0" ]; then
    echo "Validator returned zero exit code for a invalid test" 1>> validator-tests.log
    echo "Validator returned zero exit code for a invalid test. See validator-tests.log for validator comment"
    read
fi
rm -f validator-tests.log
echo "Validator test(s) finished"
