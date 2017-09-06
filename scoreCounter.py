import csv
from os import listdir
from os.path import isfile, join
from collections import Counter
fileName = input("Please enter path to machine files:")
fileList = [f for f in listdir(fileName) if isfile(join(fileName, f))]

for x in fileList:
    print("File", x, "Scores:")
    thisBoard = []
    with open(fileName+"/"+x, newline='\n') as csvfile:
        spamreader = csv.reader(csvfile, delimiter=',', quotechar='|')
        for row in spamreader:
            thisBoard.append(row[0])
            #print(', '.join(row))
    c = Counter(thisBoard)
    for y in c:
        print(y[:6] + ": " + str(c[y]))
