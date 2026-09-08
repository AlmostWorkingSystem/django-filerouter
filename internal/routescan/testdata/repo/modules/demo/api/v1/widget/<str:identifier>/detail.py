from kit.views.base import BaseAPIView


class APIView(BaseAPIView):
    def get(self, request, identifier: str):
        return None
