from rag.loader import JSONLoader

loader = JSONLoader("data/shreyaghorui222004_MedIBot.json")

data = loader.load()

print(type(data))