package authz

import "net/http"

// DebugHandler renders information about registered policies and roles.
func (ap *AuthzPlugin) DebugHandler(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	resp.Write([]byte("Authz Configuration\n"))
	resp.Write([]byte("===================\n\n\n"))

	resp.Write([]byte("Role Hierarchy\n"))
	resp.Write([]byte("--------------\n\n"))

	// Avoid picking up any roles that are known to be children,
	isChild := make(map[Role]bool)
	for child := range ap.roleParents {
		isChild[child] = true
	}

	roots := map[Role]bool{}

	// Collect potential roots from the role hierarchy.
	for _, parent := range ap.roleParents {
		if !isChild[parent] {
			roots[parent] = true
		}
	}

	// Collect potential roots from policies.
	for _, policy := range ap.policies {
		for role := range policy {
			if !isChild[role] {
				roots[role] = true
			}
		}
	}

	// Print role tree.
	tree := ap.RoleTree()
	for root := range roots {
		printTree(resp, tree, root, "", true, true)
	}

	resp.Write([]byte("\n\n\nPolicies\n"))
	resp.Write([]byte("--------\n\n"))

	padding := 20
	for action, policy := range ap.policies {
		resp.Write([]byte("  " + pad(string(action), padding) + "\n"))
		for role, effect := range policy {
			resp.Write([]byte("    " + effect.String() + " " + string(role) + "\n"))
		}
		resp.Write([]byte("\n"))
	}
}

func printTree(resp http.ResponseWriter, tree map[Role][]Role, role Role, prefix string, isRoot, isTail bool) {
	if isRoot {
		resp.Write([]byte("  " + string(role) + "\n"))
	} else {
		if isTail {
			resp.Write([]byte(prefix + "└── " + string(role) + "\n"))
		} else {
			resp.Write([]byte(prefix + "├── " + string(role) + "\n"))
		}
	}
	children := tree[role]
	for i := range len(children) - 1 {
		printTree(resp, tree, children[i], prefix+getPrefix(isRoot, isTail), false, false)
	}
	if len(children) > 0 {
		printTree(resp, tree, children[len(children)-1], prefix+getPrefix(isRoot, isTail), false, true)
	}
}

func getPrefix(isRoot, isTail bool) string {
	if isRoot {
		return "  "
	}
	if isTail {
		return "    "
	}
	return "│   "
}

func pad(str string, n int) string {
	for i := n - len(str); i > 0; i-- {
		str += " "
	}
	return str
}
