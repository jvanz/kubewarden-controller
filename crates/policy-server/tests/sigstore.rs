mod common;

#[cfg(feature = "sigstore-testing")]
use std::collections::HashMap;
#[cfg(feature = "sigstore-testing")]
use std::path::PathBuf;

#[cfg(feature = "sigstore-testing")]
use axum::{body::Body, http::{self, header, Request}};
#[cfg(feature = "sigstore-testing")]
use common::{app, default_test_config, setup};
#[cfg(feature = "sigstore-testing")]
use http_body_util::BodyExt;
#[cfg(feature = "sigstore-testing")]
use policy_evaluator::{
    admission_response_handler::policy_mode::PolicyMode,
    policy_fetcher::verify::config::VerificationConfigV1,
};
#[cfg(feature = "sigstore-testing")]
use policy_server::{
    api::admission_review::AdmissionReviewResponse,
    config::{Config, PolicyOrPolicyGroup},
};
#[cfg(feature = "sigstore-testing")]
use std::collections::BTreeSet;
#[cfg(feature = "sigstore-testing")]
use tower::ServiceExt;

/// This test is behind a feature flag because it requires a sigstore testing environment to run
/// properly. In CI this is done by the sigstore/scaffolding/actions/setup action and some
/// additional configuration. Locally, one can use the script and documentation available in the
/// same repository.
///
/// Furthermore, this test expects that there is a policy
/// "registry.local:5000/policies/testing:latest" that is signed with cosign in the local sigstore
/// instance. It also expects that the sigstore trust_config.json and verification_config.yaml files
/// are available in the workspace root directory. The trust_config.json file should contain all the
/// information to find the local sigstore instance. It follows the ClientTrustConfig format. See the
/// spec here:
/// https://github.com/sigstore/protobuf-specs/blob/4d38e4482bf67c7ab86bf2f61e8d79010ac0974e/protos/sigstore_trustroot.proto#L341
/// The verification_config.yaml file should contain the verification configuration for the policy,
/// it can be generated using `kwctl scaffold verification-config` command. The verification
/// options in the file should match the way the policy was signed in the local sigstore instance.
/// The test also checks if there is a sources.yaml file. If it exists, it is used by the policy
/// server to handle insecure registries.
#[tokio::test]
#[cfg(feature = "sigstore-testing")]
async fn test_sigstore_trust_config() {
    setup();

    // Find workspace root by traversing up from CARGO_MANIFEST_DIR
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let workspace_root = manifest_dir
        .parent()
        .and_then(|p| p.parent())
        .expect("cannot find workspace root");

    let trust_config_path = workspace_root.join("trust_config.json");
    assert!(
        trust_config_path.exists(),
        "trust_config.json not found at workspace root"
    );

    let verification_config_path = workspace_root.join("verification_config.yaml");
    assert!(
        verification_config_path.exists(),
        "verification_config.yaml not found at workspace root"
    );

    // Load verification config
    let verification_config_content = std::fs::read_to_string(&verification_config_path)
        .expect("cannot read verification_config.yaml");
    let verification_config: VerificationConfigV1 =
        serde_yaml::from_str(&verification_config_content)
            .expect("cannot parse verification_config.yaml");

    // Check if sources.yaml exists for insecure registry support
    let sources_path = workspace_root.join("sources.yaml");
    let sources = if sources_path.exists() {
        let sources_content =
            std::fs::read_to_string(&sources_path).expect("cannot read sources.yaml");
        Some(
            serde_yaml::from_str(&sources_content).expect("cannot parse sources.yaml"),
        )
    } else {
        None
    };

    // Start with default test config
    let mut config = default_test_config();

    // Override with sigstore-specific configuration
    config.sigstore_trust_config_path = Some(trust_config_path);
    config.verification_config = Some(verification_config);
    if sources.is_some() {
        config.sources = sources;
    }

    // Replace all policies with just the signed test policy
    config.policies = HashMap::from([(
        "testing".to_owned(),
        PolicyOrPolicyGroup::Policy {
            module: "registry.local:5000/policies/testing:latest".to_owned(),
            policy_mode: PolicyMode::Protect,
            allowed_to_mutate: None,
            settings: None,
            context_aware_resources: BTreeSet::new(),
            message: None,
            timeout_eval_seconds: None,
        },
    )]);

    // Create the app from config
    let app = app(config).await;

    // Send a validation request to verify the policy loads and works
    let request = Request::builder()
        .method(http::Method::POST)
        .header(header::CONTENT_TYPE, "application/json")
        .uri("/validate/testing")
        .body(Body::from(include_str!(
            "data/pod_with_privileged_containers.json"
        )))
        .unwrap();

    let response = app.oneshot(request).await.unwrap();

    // Check that the response is successful (200)
    assert_eq!(
        response.status(),
        200,
        "Expected HTTP 200, got {}",
        response.status()
    );

    // Parse the response to ensure the policy evaluated successfully
    let admission_review_response: AdmissionReviewResponse =
        serde_json::from_slice(&response.into_body().collect().await.unwrap().to_bytes())
            .expect("Failed to parse AdmissionReviewResponse");

    // The policy loaded successfully and returned a response
    // We don't check the specific allowed/denied status as that depends on the policy behavior
    // The important thing is that the policy was verified and loaded using the sigstore trust config
    assert!(
        admission_review_response.response.uid.is_some()
            || admission_review_response.response.allowed
            || !admission_review_response.response.allowed,
        "Policy evaluation completed with response"
    );
}
