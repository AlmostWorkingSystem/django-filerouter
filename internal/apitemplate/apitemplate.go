package apitemplate

import (
	"fmt"
	"regexp"
	"strings"
)

var dynamicParamRe = regexp.MustCompile(`<(\w+):(\w+)>`)

// Render ports get_api_template
// (modules/core/management/helpers/template.py) — the boilerplate scaffold
// written into a newly created, empty API view file.
func Render(path string) string {
	matches := dynamicParamRe.FindAllStringSubmatch(path, -1)
	paramsStr := ""
	if len(matches) > 0 {
		params := make([]string, 0, len(matches))
		for _, m := range matches {
			paramType, name := m[1], m[2]
			params = append(params, fmt.Sprintf("%s: %s", name, paramType))
		}
		paramsStr = ", " + strings.Join(params, ", ")
	}
	return fmt.Sprintf(apiTemplate, paramsStr, paramsStr, paramsStr)
}

const apiTemplate = `from common.request import BaseRequest
from kit.views.base import BaseAPIView
from kit.views.decorators import extend_schema
from kit.views.serializers import GenericMutationResponse
from kit.views.serializers.model import BaseModelSerializer
from kit.views.status import StatusCode
from modules.core.models.common import MasterDropdown

Model = MasterDropdown


class GetResponse(BaseModelSerializer):
    class Meta:
        model = Model
        exclude = Model.BASE_MODEL_FIELDS_EXCEPT_UUID


class PostRequest(BaseModelSerializer):
    class Meta:
        model = Model
        exclude = Model.BASE_MODEL_FIELDS


class PostResponse(GenericMutationResponse): ...


class PutRequest(BaseModelSerializer):
    class Meta:
        model = Model
        exclude = Model.BASE_MODEL_FIELDS


class PutResponse(GenericMutationResponse): ...


class APIView(BaseAPIView):
    @extend_schema(allowed_pages={})
    def get(self, request: BaseRequest%s):
        return GetResponse(), StatusCode.X_SENT_SUCCESSFUL()

    @extend_schema(allowed_pages={})
    def post(self, request: BaseRequest%s):
        body_srlz = PostRequest(data=request.data)
        body_srlz.is_valid(raise_exception=True)

        return PostResponse(), StatusCode.X_CREATE_SUCCESSFUL()

    @extend_schema(allowed_pages={})
    def put(self, request: BaseRequest%s):
        body_srlz = PutRequest(data=request.data)
        body_srlz.is_valid(raise_exception=True)

        return PutResponse(), StatusCode.X_UPDATED_SUCCESSFUL()

    `
