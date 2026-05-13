// Package resolve turns user-supplied CLI names and IDs into Render API IDs.
//
// In Render, resources can belong directly to a workspace, or they can belong
// to the ownership chain Workspace -> Project -> Environment. This package uses
// Scope to describe that chain: the workspace, project, and environment context
// that owns or contains a resource.
//
// ResolveScope is the main entry point for commands that accept some
// combination of workspace, project, and environment IDs or names. It resolves
// the provided resources and their implied ancestors, errors if the inputs
// cannot be resolved unambiguously, and falls back to the active workspace only
// when no workspace, project, or environment input is provided. It does not
// infer descendant resources that were not provided; for example, project input
// resolves the workspace and project, not a project environment.
package resolve
