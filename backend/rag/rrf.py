from collections import defaultdict


class ReciprocalRankFusion:

    def __init__(self, k: int = 60):
        self.k = k

    def fuse(self, ranked_lists):
        """
        ranked_lists: List[List[Document]]
        """

        scores = defaultdict(float)
        documents = {}

        for docs in ranked_lists:
            for rank, doc in enumerate(docs, start=1):

                key = doc.page_content

                documents[key] = doc
                scores[key] += 1 / (self.k + rank)

        ranked = sorted(
            scores.items(),
            key=lambda x: x[1],
            reverse=True,
        )

        return [documents[key] for key, _ in ranked]